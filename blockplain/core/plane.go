package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/blockplain/pubsub"
	"github.com/blockplain/config"
)

// Block represents a block in the blockchain
type Block struct {
	Height      uint64            `json:"height"`
	Hash        string            `json:"hash"`
	PrevHash    string            `json:"prev_hash"`
	PlaneID     string            `json:"plane_id"`
	Timestamp   int64             `json:"timestamp"`
	Transactions []Transaction    `json:"transactions"`
	CrossRefs   []CrossPlaneRef   `json:"cross_refs"`
	Validator   string            `json:"validator"`
	Signature   string            `json:"signature"`
	ExtraData   map[string]string `json:"extra_data"`
}

// Transaction represents a transaction in the blockchain
type Transaction struct {
	TxID        string            `json:"tx_id"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Amount      string            `json:"amount"`
	Data        []byte            `json:"data"`
	Signature   string            `json:"signature"`
	Timestamp   int64             `json:"timestamp"`
	PlaneID     string            `json:"plane_id"`
	TargetPlane string            `json:"target_plane,omitempty"`
	GasPrice    uint64            `json:"gas_price"`
	GasLimit    uint64            `json:"gas_limit"`
	Status      string            `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CrossPlaneRef represents a reference to a block in another plane
type CrossPlaneRef struct {
	PlaneID   string `json:"plane_id"`
	BlockHash string `json:"block_hash"`
	Height    uint64 `json:"height"`
	Timestamp int64  `json:"timestamp"`
}

// Plane represents a blockchain plane
type Plane struct {
	ID              string
	Config          config.PlaneConfig
	Blocks          []*Block
	PendingTxs      []Transaction
	Publisher       pubsub.Publisher
	LastHash        string
	CurrentHeight   uint64
	ValidatorPool   []string
	CurrentValidator string
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	bridgeManager   *BridgeManager
}

// NewPlane creates a new blockchain plane
func NewPlane(cfg config.PlaneConfig, publisher pubsub.Publisher) (*Plane, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	plane := &Plane{
		ID:            cfg.ID,
		Config:        cfg,
		Blocks:        make([]*Block, 0),
		PendingTxs:    make([]Transaction, 0),
		Publisher:     publisher,
		LastHash:      "",
		CurrentHeight: 0,
		ValidatorPool: cfg.Validators,
		ctx:           ctx,
		cancel:        cancel,
	}
	
	// Create Genesis block
	genesisBlock := plane.createGenesisBlock()
	plane.Blocks = append(plane.Blocks, genesisBlock)
	plane.LastHash = genesisBlock.Hash
	plane.CurrentHeight = 1
	
	// Initialize bridge manager for cross-plane operations
	plane.bridgeManager = NewBridgeManager(plane)
	
	return plane, nil
}

// Start begins the plane's operation
func (p *Plane) Start() error {
	go p.startBlockProduction()
	go p.processPendingTransactions()
	return nil
}

// Stop halts the plane's operation
func (p *Plane) Stop() {
	p.cancel()
}

// AddTransaction adds a transaction to the pending pool
func (p *Plane) AddTransaction(tx Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Basic validation
	if tx.PlaneID != p.ID && tx.TargetPlane != p.ID {
		return fmt.Errorf("transaction does not belong to this plane")
	}
	
	// Add to pending transactions
	p.PendingTxs = append(p.PendingTxs, tx)
	
	// Publish transaction to event stream
	txEvent := map[string]interface{}{
		"event_type": "new_transaction",
		"plane_id":   p.ID,
		"tx":         tx,
		"timestamp":  time.Now().Unix(),
	}
	p.Publisher.Publish("transactions", txEvent)
	
	return nil
}

// createGenesisBlock creates the first block in the chain
func (p *Plane) createGenesisBlock() *Block {
	timestamp := time.Now().Unix()
	block := &Block{
		Height:       0,
		PlaneID:      p.ID,
		Timestamp:    timestamp,
		Transactions: []Transaction{},
		CrossRefs:    []CrossPlaneRef{},
		ExtraData:    map[string]string{"genesis": "true"},
	}
	
	blockData, _ := json.Marshal(block)
	hash := sha256.Sum256(blockData)
	block.Hash = hex.EncodeToString(hash[:])
	
	return block
}

// createBlock creates a new block with pending transactions
func (p *Plane) createBlock() *Block {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Select pending transactions (up to block limit)
	var blockTxs []Transaction
	txLimit := p.Config.BlockTxLimit
	if txLimit > len(p.PendingTxs) {
		txLimit = len(p.PendingTxs)
	}
	
	blockTxs = p.PendingTxs[:txLimit]
	p.PendingTxs = p.PendingTxs[txLimit:]
	
	// Get cross-plane references
	crossRefs := p.bridgeManager.GetCrossPlaneRefs()
	
	// Create new block
	block := &Block{
		Height:       p.CurrentHeight,
		PrevHash:     p.LastHash,
		PlaneID:      p.ID,
		Timestamp:    time.Now().Unix(),
		Transactions: blockTxs,
		CrossRefs:    crossRefs,
		Validator:    p.CurrentValidator,
		ExtraData:    make(map[string]string),
	}
	
	// Calculate block hash
	blockData, _ := json.Marshal(block)
	hash := sha256.Sum256(blockData)
	block.Hash = hex.EncodeToString(hash[:])
	
	return block
}

// startBlockProduction begins producing blocks at the configured interval
func (p *Plane) startBlockProduction() {
	ticker := time.NewTicker(time.Duration(p.Config.BlockTime) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Only create a block if there are pending transactions or cross-references
			if len(p.PendingTxs) > 0 || p.bridgeManager.HasPendingRefs() {
				p.produceBlock()
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// produceBlock creates and adds a new block to the chain
func (p *Plane) produceBlock() {
	// Select validator (round-robin for simplicity)
	p.rotateValidator()
	
	// Create new block
	newBlock := p.createBlock()
	
	// Add block to chain
	p.mu.Lock()
	p.Blocks = append(p.Blocks, newBlock)
	p.LastHash = newBlock.Hash
	p.CurrentHeight++
	p.mu.Unlock()
	
	// Publish block to event stream
	blockEvent := map[string]interface{}{
		"event_type": "new_block",
		"plane_id":   p.ID,
		"block":      newBlock,
		"timestamp":  time.Now().Unix(),
	}
	p.Publisher.Publish("blocks", blockEvent)
	
	// Process cross-plane transactions
	for _, tx := range newBlock.Transactions {
		if tx.TargetPlane != "" && tx.TargetPlane != p.ID {
			p.bridgeManager.SendCrossPlaneTransaction(tx)
		}
	}
}

// rotateValidator selects the next validator in the pool
func (p *Plane) rotateValidator() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if len(p.ValidatorPool) == 0 {
		p.CurrentValidator = "default-validator"
		return
	}
	
	// Find current validator index
	currentIdx := 0
	for i, validator := range p.ValidatorPool {
		if validator == p.CurrentValidator {
			currentIdx = i
			break
		}
	}
	
	// Select next validator
	nextIdx := (currentIdx + 1) % len(p.ValidatorPool)
	p.CurrentValidator = p.ValidatorPool[nextIdx]
}

// processPendingTransactions continuously processes incoming transactions
func (p *Plane) processPendingTransactions() {
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			// Nothing to do, just wait for new transactions
		case <-p.ctx.Done():
			return
		}
	}
}

// GetBlock retrieves a block by its hash
func (p *Plane) GetBlock(hash string) (*Block, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	for _, block := range p.Blocks {
		if block.Hash == hash {
			return block, nil
		}
	}
	
	return nil, fmt.Errorf("block not found")
}

// GetTransaction retrieves a transaction by its ID
func (p *Plane) GetTransaction(txID string) (*Transaction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Check pending transactions first
	for i, tx := range p.PendingTxs {
		if tx.TxID == txID {
			return &p.PendingTxs[i], nil
		}
	}
	
	// Check transactions in blocks
	for _, block := range p.Blocks {
		for i, tx := range block.Transactions {
			if tx.TxID == txID {
				return &block.Transactions[i], nil
			}
		}
	}
	
	return nil, fmt.Errorf("transaction not found")
}

// Bridge manager for handling cross-plane communication
type BridgeManager struct {
	plane         *Plane
	pendingRefs   []CrossPlaneRef
	pendingTxs    []Transaction
	mu            sync.RWMutex
}

// NewBridgeManager creates a new bridge manager
func NewBridgeManager(plane *Plane) *BridgeManager {
	return &BridgeManager{
		plane:       plane,
		pendingRefs: make([]CrossPlaneRef, 0),
		pendingTxs:  make([]Transaction, 0),
	}
}

// GetCrossPlaneRefs returns pending cross-plane references
func (bm *BridgeManager) GetCrossPlaneRefs() []CrossPlaneRef {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	refs := bm.pendingRefs
	bm.pendingRefs = make([]CrossPlaneRef, 0)
	return refs
}

// HasPendingRefs checks if there are pending cross-plane references
func (bm *BridgeManager) HasPendingRefs() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return len(bm.pendingRefs) > 0
}

// AddCrossPlaneRef adds a reference to another plane
func (bm *BridgeManager) AddCrossPlaneRef(ref CrossPlaneRef) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.pendingRefs = append(bm.pendingRefs, ref)
	
	// Publish bridge event
	bridgeEvent := map[string]interface{}{
		"event_type":  "cross_plane_ref",
		"source_plane": bm.plane.ID,
		"target_plane": ref.PlaneID,
		"ref":          ref,
		"timestamp":    time.Now().Unix(),
	}
	bm.plane.Publisher.Publish("bridges", bridgeEvent)
}

// SendCrossPlaneTransaction forwards a transaction to another plane
func (bm *BridgeManager) SendCrossPlaneTransaction(tx Transaction) {
	// In a real system, this would communicate with other planes
	// For now, just publish the cross-plane transaction event
	bridgeTxEvent := map[string]interface{}{
		"event_type":   "cross_plane_tx",
		"source_plane": bm.plane.ID,
		"target_plane": tx.TargetPlane,
		"tx":           tx,
		"timestamp":    time.Now().Unix(),
	}
	bm.plane.Publisher.Publish("bridges", bridgeTxEvent)
}

// ReceiveCrossPlaneTransaction handles an incoming transaction from another plane
func (bm *BridgeManager) ReceiveCrossPlaneTransaction(tx Transaction) error {
	// Validate the cross-plane transaction
	if tx.TargetPlane != bm.plane.ID {
		return fmt.Errorf("transaction target plane mismatch")
	}
	
	// Add the transaction to this plane
	return bm.plane.AddTransaction(tx)
}

// CrossPlaneManager coordinates communication between planes
type CrossPlaneManager struct {
	planes     map[string]*Plane
	publisher  pubsub.Publisher
	mu         sync.RWMutex
}

// NewCrossPlaneManager creates a new cross-plane manager
func NewCrossPlaneManager(planes map[string]*Plane, publisher pubsub.Publisher) *CrossPlaneManager {
	return &CrossPlaneManager{
		planes:    planes,
		publisher: publisher,
	}
}

// AddPlane adds a new plane to the manager
func (cpm *CrossPlaneManager) AddPlane(plane *Plane) {
	cpm.mu.Lock()
	defer cpm.mu.Unlock()
	cpm.planes[plane.ID] = plane
}

// RemovePlane removes a plane from the manager
func (cpm *CrossPlaneManager) RemovePlane(planeID string) {
	cpm.mu.Lock()
	defer cpm.mu.Unlock()
	delete(cpm.planes, planeID)
}

// ForwardTransaction forwards a transaction to its target plane
func (cpm *CrossPlaneManager) ForwardTransaction(tx Transaction) error {
	cpm.mu.RLock()
	defer cpm.mu.RUnlock()
	
	targetPlane, exists := cpm.planes[tx.TargetPlane]
	if !exists {
		return fmt.Errorf("target plane %s not found", tx.TargetPlane)
	}
	
	return targetPlane.bridgeManager.ReceiveCrossPlaneTransaction(tx)
}

// SyncPlaneState exchanges state information between planes
func (cpm *CrossPlaneManager) SyncPlaneState() {
	cpm.mu.RLock()
	defer cpm.mu.RUnlock()
	
	// For each plane, create cross-references to the latest blocks in other planes
	for planeID, plane := range cpm.planes {
		for otherID, otherPlane := range cpm.planes {
			if planeID == otherID {
				continue
			}
			
			// Create a cross-reference to the latest block in the other plane
			otherPlane.mu.RLock()
			if len(otherPlane.Blocks) > 0 {
				latestBlock := otherPlane.Blocks[len(otherPlane.Blocks)-1]
				ref := CrossPlaneRef{
					PlaneID:   otherID,
					BlockHash: latestBlock.Hash,
					Height:    latestBlock.Height,
					Timestamp: latestBlock.Timestamp,
				}
				otherPlane.mu.RUnlock()
				
				plane.bridgeManager.AddCrossPlaneRef(ref)
			} else {
				otherPlane.mu.RUnlock()
			}
		}
	}
}

// GetPlaneState returns the current state of all planes
func (cpm *CrossPlaneManager) GetPlaneState() map[string]interface{} {
	cpm.mu.RLock()
	defer cpm.mu.RUnlock()
	
	state := make(map[string]interface{})
	for id, plane := range cpm.planes {
		plane.mu.RLock()
		planeState := map[string]interface{}{
			"id":             id,
			"current_height": plane.CurrentHeight,
			"last_hash":      plane.LastHash,
			"validator":      plane.CurrentValidator,
			"block_count":    len(plane.Blocks),
			"pending_txs":    len(plane.PendingTxs),
		}
		plane.mu.RUnlock()
		
		state[id] = planeState
	}
	
	return state
}