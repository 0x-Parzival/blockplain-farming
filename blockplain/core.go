package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Block represents a block in the blockchain
type Block struct {
	Height        uint64            `json:"height"`
	Timestamp     int64             `json:"timestamp"`
	PreviousHash  string            `json:"previous_hash"`
	Hash          string            `json:"hash"`
	Transactions  []Transaction     `json:"transactions"`
	PlaneID       string            `json:"plane_id"`
	CrossRefs     map[string]string `json:"cross_refs"` // References to blocks in other planes
	ValidatorSigs []ValidatorSig    `json:"validator_signatures"`
}

// Transaction represents a transaction within a block
type Transaction struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Amount    float64                `json:"amount"`
	Asset     string                 `json:"asset"`
	Timestamp int64                  `json:"timestamp"`
	Signature string                 `json:"signature"`
	Metadata  map[string]interface{} `json:"metadata"`
	PlaneID   string                 `json:"plane_id"`
	IsCross   bool                   `json:"is_cross_chain"` // Indicates if this is a cross-chain transaction
}

// ValidatorSig represents a validator's signature on a block
type ValidatorSig struct {
	ValidatorID string `json:"validator_id"`
	Signature   string `json:"signature"`
	Timestamp   int64  `json:"timestamp"`
}

// Blockchain represents the main blockchain data structure
type Blockchain struct {
	Planes map[string]*BlockchainPlane `json:"planes"`
	mu     sync.RWMutex
}

// BlockchainPlane represents a single plane in the blockplain
type BlockchainPlane struct {
	ID            string            `json:"id"`
	Blocks        []*Block          `json:"blocks"`
	Validators    []string          `json:"validators"`
	Connections   map[string]bool   `json:"connections"` // Connected plane IDs
	CurrentHeight uint64            `json:"current_height"`
	mu            sync.RWMutex
}

// NewBlockchain creates a new blockchain with the specified planes
func NewBlockchain(planeIDs []string) *Blockchain {
	bc := &Blockchain{
		Planes: make(map[string]*BlockchainPlane),
	}

	// Initialize each plane with a genesis block
	for _, id := range planeIDs {
		plane := &BlockchainPlane{
			ID:            id,
			Blocks:        make([]*Block, 0),
			Validators:    make([]string, 0),
			Connections:   make(map[string]bool),
			CurrentHeight: 0,
		}

		// Create genesis block for this plane
		genesisBlock := createGenesisBlock(id)
		plane.Blocks = append(plane.Blocks, genesisBlock)
		plane.CurrentHeight = 1

		bc.Planes[id] = plane
	}

	// Connect planes (full mesh connectivity)
	for _, plane := range bc.Planes {
		for otherID := range bc.Planes {
			if otherID != plane.ID {
				plane.Connections[otherID] = true
			}
		}
	}

	return bc
}

// createGenesisBlock creates a genesis block for a specific plane
func createGenesisBlock(planeID string) *Block {
	timestamp := time.Now().Unix()
	genesisBlock := &Block{
		Height:        0,
		Timestamp:     timestamp,
		PreviousHash:  "0",
		PlaneID:       planeID,
		CrossRefs:     make(map[string]string),
		Transactions:  []Transaction{},
		ValidatorSigs: []ValidatorSig{},
	}

	// Calculate hash
	genesisBlock.Hash = calculateBlockHash(genesisBlock)

	return genesisBlock
}

// calculateBlockHash generates a SHA-256 hash of the block
func calculateBlockHash(block *Block) string {
	blockData, _ := json.Marshal(struct {
		Height        uint64
		Timestamp     int64
		PreviousHash  string
		PlaneID       string
		CrossRefs     map[string]string
		Transactions  []Transaction
		ValidatorSigs []ValidatorSig
	}{
		Height:        block.Height,
		Timestamp:     block.Timestamp,
		PreviousHash:  block.PreviousHash,
		PlaneID:       block.PlaneID,
		CrossRefs:     block.CrossRefs,
		Transactions:  block.Transactions,
		ValidatorSigs: block.ValidatorSigs,
	})

	hash := sha256.Sum256(blockData)
	return hex.EncodeToString(hash[:])
}

// AddBlock adds a new block to the specified plane
func (bc *Blockchain) AddBlock(planeID string, transactions []Transaction) (*Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return nil, fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.Lock()
	defer plane.mu.Unlock()

	previousBlock := plane.Blocks[len(plane.Blocks)-1]

	newBlock := &Block{
		Height:        previousBlock.Height + 1,
		Timestamp:     time.Now().Unix(),
		PreviousHash:  previousBlock.Hash,
		PlaneID:       planeID,
		CrossRefs:     make(map[string]string),
		Transactions:  transactions,
		ValidatorSigs: []ValidatorSig{},
	}

	// Add cross-references to the latest blocks in connected planes
	for connectedPlaneID := range plane.Connections {
		if connectedPlane, ok := bc.Planes[connectedPlaneID]; ok {
			connectedPlane.mu.RLock()
			if len(connectedPlane.Blocks) > 0 {
				latestBlock := connectedPlane.Blocks[len(connectedPlane.Blocks)-1]
				newBlock.CrossRefs[connectedPlaneID] = latestBlock.Hash
			}
			connectedPlane.mu.RUnlock()
		}
	}

	// Calculate hash for the new block
	newBlock.Hash = calculateBlockHash(newBlock)

	// Add block to the chain
	plane.Blocks = append(plane.Blocks, newBlock)
	plane.CurrentHeight = newBlock.Height

	return newBlock, nil
}

// GetLatestBlock returns the latest block from the specified plane
func (bc *Blockchain) GetLatestBlock(planeID string) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return nil, fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.RLock()
	defer plane.mu.RUnlock()

	if len(plane.Blocks) == 0 {
		return nil, fmt.Errorf("no blocks in plane: %s", planeID)
	}

	return plane.Blocks[len(plane.Blocks)-1], nil
}

// GetBlock returns a specific block by its height from the specified plane
func (bc *Blockchain) GetBlock(planeID string, height uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return nil, fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.RLock()
	defer plane.mu.RUnlock()

	if height >= uint64(len(plane.Blocks)) {
		return nil, fmt.Errorf("block height %d out of range for plane %s", height, planeID)
	}

	return plane.Blocks[height], nil
}

// AddValidator adds a validator to the specified plane
func (bc *Blockchain) AddValidator(planeID, validatorID string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.Lock()
	defer plane.mu.Unlock()

	// Check if validator already exists
	for _, id := range plane.Validators {
		if id == validatorID {
			return fmt.Errorf("validator already exists: %s", validatorID)
		}
	}

	plane.Validators = append(plane.Validators, validatorID)
	return nil
}

// RemoveValidator removes a validator from the specified plane
func (bc *Blockchain) RemoveValidator(planeID, validatorID string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.Lock()
	defer plane.mu.Unlock()

	found := false
	for i, id := range plane.Validators {
		if id == validatorID {
			// Remove validator by swapping with the last element and then truncating
			lastIndex := len(plane.Validators) - 1
			plane.Validators[i] = plane.Validators[lastIndex]
			plane.Validators = plane.Validators[:lastIndex]
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("validator not found: %s", validatorID)
	}

	return nil
}

// AddCrossChainTransaction creates a transaction that spans multiple planes
func (bc *Blockchain) AddCrossChainTransaction(fromPlaneID, toPlaneID string, tx Transaction) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Check if both planes exist
	_, fromExists := bc.Planes[fromPlaneID]
	_, toExists := bc.Planes[toPlaneID]

	if !fromExists || !toExists {
		return fmt.Errorf("one or both planes do not exist: %s, %s", fromPlaneID, toPlaneID)
	}

	// Mark transaction as cross-chain
	tx.IsCross = true
	
	// Create a copy of the transaction for each plane
	fromTx := tx
	fromTx.PlaneID = fromPlaneID
	
	toTx := tx
	toTx.PlaneID = toPlaneID

	// Add to source plane
	_, err := bc.AddBlock(fromPlaneID, []Transaction{fromTx})
	if err != nil {
		return err
	}

	// Add to destination plane
	_, err = bc.AddBlock(toPlaneID, []Transaction{toTx})
	if err != nil {
		return err
	}

	return nil
}

// ValidateChain checks if the blockchain is valid by verifying all block hashes
func (bc *Blockchain) ValidateChain(planeID string) (bool, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	plane, exists := bc.Planes[planeID]
	if !exists {
		return false, fmt.Errorf("plane does not exist: %s", planeID)
	}

	plane.mu.RLock()
	defer plane.mu.RUnlock()

	for i := 1; i < len(plane.Blocks); i++ {
		currentBlock := plane.Blocks[i]
		previousBlock := plane.Blocks[i-1]

		// Check if the previous hash matches
		if currentBlock.PreviousHash != previousBlock.Hash {
			return false, fmt.Errorf("previous hash mismatch at height %d", currentBlock.Height)
		}

		// Recalculate and check the hash
		calculatedHash := calculateBlockHash(currentBlock)
		if calculatedHash != currentBlock.Hash {
			return false, fmt.Errorf("hash mismatch at height %d", currentBlock.Height)
		}
	}

	return true, nil
}