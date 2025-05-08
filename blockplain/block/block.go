// blockplain/block/block.go

package block

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Block represents a basic block in the blockchain
type Block struct {
	Index        uint64            `json:"index"`
	Timestamp    int64             `json:"timestamp"`
	Data         []byte            `json:"data"`
	PreviousHash string            `json:"previous_hash"`
	Hash         string            `json:"hash"`
	Metadata     map[string]string `json:"metadata"`
	Signature    []byte            `json:"signature,omitempty"`
}

// NewBlock creates a new block with the given data
func NewBlock(index uint64, data []byte, previousHash string) *Block {
	block := &Block{
		Index:        index,
		Timestamp:    time.Now().Unix(),
		Data:         data,
		PreviousHash: previousHash,
		Metadata:     make(map[string]string),
	}
	
	block.Hash = block.CalculateHash()
	return block
}

// CalculateHash generates a SHA256 hash of the block
func (b *Block) CalculateHash() string {
	record := fmt.Sprintf("%d%d%s%s", b.Index, b.Timestamp, b.Data, b.PreviousHash)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// Serialize converts the block to JSON
func (b *Block) Serialize() ([]byte, error) {
	return json.Marshal(b)
}

// Deserialize converts JSON to a block
func Deserialize(data []byte) (*Block, error) {
	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// Validate checks if the block is valid
func (b *Block) Validate() bool {
	return b.Hash == b.CalculateHash()
}

// SetMetadata adds a key-value pair to the block's metadata
func (b *Block) SetMetadata(key, value string) {
	b.Metadata[key] = value
}

// GetMetadata retrieves a value from the block's metadata
func (b *Block) GetMetadata(key string) (string, bool) {
	value, exists := b.Metadata[key]
	return value, exists
}