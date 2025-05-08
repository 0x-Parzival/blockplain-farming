// blockplain/main.go (updated)

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/blockplain/api"
	"github.com/yourusername/blockplain/blockchain"
	"github.com/yourusername/blockplain/config"
	"github.com/yourusername/blockplain/network"
	"github.com/yourusername/blockplain/storage"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()
	
	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	
	// Initialize storage
	store, err := storage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Check if blockchain exists
	// blockplain/main.go (continued)

	// Check if blockchain exists
	length, _, err := store.LoadChainInfo()
	if err != nil {
		log.Printf("Failed to load chain info: %v", err)
	}
	
	var chain *blockchain.Blockchain
	if length == 0 {
		// Create new blockchain with genesis block
		log.Println("Creating new blockchain with genesis block")
		genesisData := []byte("BlockPlain Genesis Block - " + cfg.NodeID)
		chain = blockchain.New(genesisData, cfg.NodeID)
		
		// Save genesis block
		if err := store.SaveBlock(chain.GetLatestBlock()); err != nil {
			log.Fatalf("Failed to save genesis block: %v", err)
		}
		
		// Save chain info
		if err := store.SaveChainInfo(1, chain.GetLatestBlock().Hash); err != nil {
			log.Fatalf("Failed to save chain info: %v", err)
		}
	} else {
		// Load existing blockchain
		log.Printf("Loading existing blockchain with %d blocks", length)
		blocks, err := store.LoadAllBlocks()
		if err != nil {
			log.Fatalf("Failed to load blocks: %v", err)
		}
		
		// Create blockchain with loaded blocks
		chain = &blockchain.Blockchain{
			Blocks: blocks,
		}
		
		// Validate chain
		if !chain.ValidateChain() {
			log.Fatalf("Loaded blockchain is invalid")
		}
		
		log.Printf("Loaded blockchain with %d blocks", chain.GetLength())
	}
	
	// Start API server
	server := api.New(chain, cfg.APIPort)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("API server failed: %v", err)
		}
	}()
	
	// Setup P2P network
	net := network.New(chain, cfg.P2PPort, cfg.NodeID, cfg.SyncInterval)
	if err := net.Start(); err != nil {
		log.Fatalf("Failed to start P2P network: %v", err)
	}
	
	// Add initial peers
	for _, peerAddr := range cfg.PeerList {
		net.AddPeer(peerAddr)
	}
	
	// Create block saver
	go func() {
		lastSavedIndex := chain.GetLatestBlock().Index
		for {
			time.Sleep(5 * time.Second)
			
			latestBlock := chain.GetLatestBlock()
			if latestBlock.Index > lastSavedIndex {
				// Save new blocks
				for i := lastSavedIndex + 1; i <= latestBlock.Index; i++ {
					block, err := chain.GetBlockByIndex(i)
					if err != nil {
						log.Printf("Failed to get block %d: %v", i, err)
						continue
					}
					
					if err := store.SaveBlock(block); err != nil {
						log.Printf("Failed to save block %d: %v", i, err)
					}
				}
				
				// Update chain info
				if err := store.SaveChainInfo(uint64(chain.GetLength()), latestBlock.Hash); err != nil {
					log.Printf("Failed to save chain info: %v", err)
				}
				
				lastSavedIndex = latestBlock.Index
			}
		}
	}()
	
	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	log.Println("Shutting down...")
	
	// Save chain info before exit
	if err := store.SaveChainInfo(uint64(chain.GetLength()), chain.GetLatestBlock().Hash); err != nil {
		log.Printf("Failed to save chain info: %v", err)
	}
	
	log.Println("Shutdown complete")
}