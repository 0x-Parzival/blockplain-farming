// blockplain/api/server.go

package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/yourusername/blockplain/blockchain"
)

// Server represents the API server
type Server struct {
	blockchain *blockchain.Blockchain
	port       int
}

// New creates a new API server
func New(blockchain *blockchain.Blockchain, port int) *Server {
	return &Server{
		blockchain: blockchain,
		port:       port,
	}
}

// Start starts the API server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/blocks", s.handleGetBlocks)
	mux.HandleFunc("/block", s.handleGetBlock)
	mux.HandleFunc("/block/latest", s.handleGetLatestBlock)
	mux.HandleFunc("/block/add", s.handleAddBlock)
	mux.HandleFunc("/chain/validate", s.handleValidateChain)
	
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleGetBlocks returns all blocks in the chain
func (s *Server) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	writeJSON(w, s.blockchain.Blocks)
}

// handleGetBlock returns a specific block by index or hash
func (s *Server) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	query := r.URL.Query()
	
	// Get block by index
	if idxStr := query.Get("index"); idxStr != "" {
		idx, err := strconv.ParseUint(idxStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid index", http.StatusBadRequest)
			return
		}
		
		block, err := s.blockchain.GetBlockByIndex(idx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		writeJSON(w, block)
		return
	}
	
	// Get block by hash
	if hash := query.Get("hash"); hash != "" {
		block, err := s.blockchain.GetBlockByHash(hash)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		writeJSON(w, block)
		return
	}
	
	http.Error(w, "Missing index or hash parameter", http.StatusBadRequest)
}

// handleGetLatestBlock returns the latest block in the chain
func (s *Server) handleGetLatestBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	block := s.blockchain.GetLatestBlock()
	if block == nil {
		http.Error(w, "No blocks in chain", http.StatusNotFound)
		return
	}
	
	writeJSON(w, block)
}

// handleAddBlock adds a new block to the chain
func (s *Server) handleAddBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	block, err := s.blockchain.AddBlock(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	writeJSON(w, block)
}

// handleValidateChain validates the blockchain
func (s *Server) handleValidateChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	valid := s.blockchain.ValidateChain()
	writeJSON(w, map[string]bool{"valid": valid})
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}