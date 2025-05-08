// blockplain/network/peer.go

package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/blockplain/block"
	"github.com/yourusername/blockplain/blockchain"
)

// Peer represents a network peer
type Peer struct {
	Address     string
	LastSeen    time.Time
	BlockHeight uint64
}

// Network handles peer-to-peer communication
type Network struct {
	blockchain  *blockchain.Blockchain
	peers       map[string]*Peer
	mutex       sync.RWMutex
	port        int
	nodeID      string
	syncInterval time.Duration
}

// New creates a new network instance
func New(blockchain *blockchain.Blockchain, port int, nodeID string, syncInterval time.Duration) *Network {
	return &Network{
		blockchain:   blockchain,
		peers:        make(map[string]*Peer),
		port:         port,
		nodeID:       nodeID,
		syncInterval: syncInterval,
	}
}

// Start starts the network server and synchronization
func (n *Network) Start() error {
	// Start HTTP server for peer communication
	mux := http.NewServeMux()
	
	mux.HandleFunc("/peers", n.handlePeers)
	mux.HandleFunc("/blocks", n.handleBlocks)
	mux.HandleFunc("/blocks/latest", n.handleLatestBlock)
	
	addr := fmt.Sprintf(":%d", n.port)
	go func() {
		log.Printf("Starting P2P server on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("P2P server failed: %v", err)
		}
	}()
	
	// Start synchronization loop
	go n.syncLoop()
	
	return nil
}

// AddPeer adds a peer to the network
func (n *Network) AddPeer(address string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	
	// Normalize address
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	
	n.peers[address] = &Peer{
		Address:     address,
		LastSeen:    time.Now(),
		BlockHeight: 0,
	}
	
	log.Printf("Added peer: %s", address)
}

// RemovePeer removes a peer from the network
func (n *Network) RemovePeer(address string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	
	delete(n.peers, address)
	log.Printf("Removed peer: %s", address)
}

// GetPeers returns a list of active peers
func (n *Network) GetPeers() []*Peer {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	
	peers := make([]*Peer, 0, len(n.peers))
	for _, peer := range n.peers {
		peers = append(peers, peer)
	}
	
	return peers
}

// syncLoop periodically synchronizes with peers
func (n *Network) syncLoop() {
	ticker := time.NewTicker(n.syncInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		n.synchronize()
	}
}

// synchronize synchronizes with all peers
func (n *Network) synchronize() {
	n.mutex.RLock()
	peers := make([]*Peer, 0, len(n.peers))
	for _, peer := range n.peers {
		peers = append(peers, peer)
	}
	n.mutex.RUnlock()
	
	for _, peer := range peers {
		go n.syncWithPeer(peer)
	}
}

// syncWithPeer synchronizes with a single peer
func (n *Network) syncWithPeer(peer *Peer) {
	// Get latest block from peer
	resp, err := http.Get(fmt.Sprintf("%s/blocks/latest", peer.Address))
	if err != nil {
		log.Printf("Failed to get latest block from peer %s: %v", peer.Address, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Peer %s returned status %d", peer.Address, resp.StatusCode)
		return
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response from peer %s: %v", peer.Address, err)
		return
	}
	
	var latestBlock block.Block
	if err := json.Unmarshal(body, &latestBlock); err != nil {
		log.Printf("Failed to parse latest block from peer %s: %v", peer.Address, err)
		return
	}
	
	// Update peer info
	n.mutex.Lock()
	if p, exists := n.peers[peer.Address]; exists {
		p.LastSeen = time.Now()
		p.BlockHeight = latestBlock.Index
	}
	n.mutex.Unlock()
	
	// Check if peer has more blocks than us
	ourLatest := n.blockchain.GetLatestBlock()
	if latestBlock.Index <= ourLatest.Index {
		return
	}
	
	log.Printf("Peer %s has newer blocks (%d vs %d), syncing...", 
		peer.Address, latestBlock.Index, ourLatest.Index)
	
	// Get all blocks from peer
	resp, err = http.Get(fmt.Sprintf("%s/blocks", peer.Address))
	if err != nil {
		log.Printf("Failed to get blocks from peer %s: %v", peer.Address, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Peer %s returned status %d when getting blocks", peer.Address, resp.StatusCode)
		return
	}
	
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read blocks response from peer %s: %v", peer.Address, err)
		return
	}
	
	var blocks []*block.Block
	if err := json.Unmarshal(body, &blocks); err != nil {
		log.Printf("Failed to parse blocks from peer %s: %v", peer.Address, err)
		return
	}
	
	// Create new blockchain with peer's blocks
	newChain := &blockchain.Blockchain{
		Blocks: blocks,
	}
	
	// Validate and replace our chain if peer's is valid and longer
	if err := n.blockchain.ReplaceChain(newChain); err != nil {
		log.Printf("Failed to replace chain with peer's chain: %v", err)
		return
	}
	
	log.Printf("Successfully synced %d blocks from peer %s", len(blocks), peer.Address)
}

// handlePeers handles peer list requests
func (n *Network) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Return list of peers
		peers := n.GetPeers()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(peers)
	} else if r.Method == http.MethodPost {
		// Add new peer
		var peer struct {
			Address string `json:"address"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		n.AddPeer(peer.Address)
		w.WriteHeader(http.StatusCreated)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBlocks handles block list requests
func (n *Network) handleBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n.blockchain.Blocks)
}

// handleLatestBlock handles latest block requests
func (n *Network) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	block := n.blockchain.GetLatestBlock()
	if block == nil {
		http.Error(w, "No blocks in chain", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(block)
}