// blockplain/block/signed_block.go

package block

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	"github.com/yourusername/blockplain/crypto"
)

// SignBlock signs a block with the provided key pair
func SignBlock(block *Block, keyPair *crypto.KeyPair) error {
	// Get block data without signature
	block.Signature = nil
	data, err := block.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize block: %v", err)
	}
	
	// Sign block data
	signature, err := keyPair.Sign(data)
	if err != nil {
		return fmt.Errorf("failed to sign block: %v", err)
	}
	
	// Set signature
	block.Signature = signature
	return nil
}

// VerifyBlockSignature verifies a block's signature
func VerifyBlockSignature(block *Block, publicKey *ecdsa.PublicKey) bool {
	if block.Signature == nil {
		return false
	}
	
	// Get signature
	signature := block.Signature
	
	// Clear signature for verification
	block.Signature = nil
	data, err := block.Serialize()
	if err != nil {
		return false
	}
	
	// Restore signature
	block.Signature = signature
	
	// Verify signature
	return crypto.Verify(publicKey, data, signature)
}

// BlockWithProof represents a block with proof of validity
type BlockWithProof struct {
	Block      *Block `json:"block"`
	NodeID     string `json:"node_id"`
	ValidProof bool   `json:"valid_proof"`
}

// NewBlockWithProof creates a new block with proof
func NewBlockWithProof(block *Block, nodeID string, keyPair *crypto.KeyPair) (*BlockWithProof, error) {
	// Create a copy of the block
	blockCopy := &Block{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Data:         block.Data,
		PreviousHash: block.PreviousHash,
		Hash:         block.Hash,
		Metadata:     make(map[string]string),
	}
	
	// Copy metadata
	for k, v := range block.Metadata {
		blockCopy.Metadata[k] = v
	}
	
	// Add node ID to metadata
	blockCopy.SetMetadata("node_id", nodeID)
	
	// Sign block
	if err := SignBlock(blockCopy, keyPair); err != nil {
		return nil, err
	}
	
	return &BlockWithProof{
		Block:      blockCopy,
		NodeID:     nodeID,
		ValidProof: true,
	}, nil
}

// Verify checks if the block proof is valid
func (bp *BlockWithProof) Verify(publicKey *ecdsa.PublicKey) bool {
	// Verify block hash
	if !bp.Block.Validate() {
		return false
	}
	
	// Verify signature
	return VerifyBlockSignature(bp.Block, publicKey)
}

// Serialize converts the block with proof to JSON
func (bp *BlockWithProof) Serialize() ([]byte, error) {
	return json.Marshal(bp)
}

// DeserializeBlockWithProof converts JSON to block with proof
func DeserializeBlockWithProof(data []byte) (*BlockWithProof, error) {
	var bp BlockWithProof
	if err := json.Unmarshal(data, &bp); err != nil {
		return nil, err
	}
	return &bp, nil
}