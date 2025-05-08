// blockplain/blockchain/blockchain.go

package blockchain

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/blockplain/block"
)

// Blockchain represents a chain of blocks
type Blockchain struct {
	Blocks     []*block.Block
	mutex      sync.RWMutex
	genesisKey string
}

// New creates a new blockchain with genesis block
func New(genesisData []byte, genesisKey string) *Blockchain {
	genesis := block.NewBlock(0, genesisData, "0")
	genesis.SetMetadata("genesis", "true")
	genesis.SetMetadata("timestamp", time.Now().Format(time.RFC3339))
	
	chain := &Blockchain{
		Blocks:     []*block.Block{genesis},
		genesisKey: genesisKey,
	}
	
	return chain
}

// GetLatestBlock returns the most recent block in the chain
func (bc *Blockchain) GetLatestBlock() *block.Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// AddBlock adds a new block to the chain
func (bc *Blockchain) AddBlock(data []byte) (*block.Block, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	
	prevBlock := bc.Blocks[len(bc.Blocks)-1]
	newBlock := block.NewBlock(prevBlock.Index+1, data, prevBlock.Hash)
	
	bc.Blocks = append(bc.Blocks, newBlock)
	return newBlock, nil
}

// GetBlockByIndex returns a block by its index
func (bc *Blockchain) GetBlockByIndex(index uint64) (*block.Block, error) {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	for _, block := range bc.Blocks {
		if block.Index == index {
			return block, nil
		}
	}
	
	return nil, fmt.Errorf("block with index %d not found", index)
}

// GetBlockByHash returns a block by its hash
func (bc *Blockchain) GetBlockByHash(hash string) (*block.Block, error) {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	for _, block := range bc.Blocks {
		if block.Hash == hash {
			return block, nil
		}
	}
	
	return nil, fmt.Errorf("block with hash %s not found", hash)
}

// ValidateChain checks if the blockchain is valid
func (bc *Blockchain) ValidateChain() bool {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	for i := 1; i < len(bc.Blocks); i++ {
		currentBlock := bc.Blocks[i]
		previousBlock := bc.Blocks[i-1]
		
		// Validate hash
		if currentBlock.Hash != currentBlock.CalculateHash() {
			return false
		}
		
		// Validate previous hash reference
		if currentBlock.PreviousHash != previousBlock.Hash {
			return false
		}
	}
	
	return true
}

// GetLength returns the number of blocks in the chain
func (bc *Blockchain) GetLength() int {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	return len(bc.Blocks)
}

// ReplaceChain replaces the current chain with a new one if it's valid and longer
func (bc *Blockchain) ReplaceChain(newChain *Blockchain) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	
	// Check if new chain is longer
	if len(newChain.Blocks) <= len(bc.Blocks) {
		return errors.New("new chain is not longer than current chain")
	}
	
	// Check if new chain is valid
	if !newChain.ValidateChain() {
		return errors.New("new chain is invalid")
	}
	
	// Check if new chain has the same genesis block
	if newChain.Blocks[0].GetMetadata("genesis", "") != bc.Blocks[0].GetMetadata("genesis", "") {
		return errors.New("new chain has different genesis block")
	}
	
	bc.Blocks = newChain.Blocks
	return nil
}