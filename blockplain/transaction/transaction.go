// blockplain/transaction/transaction.go

package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"https://github.com/0x-Parzival/blockplain-farming/blockplain/crypto"
)

// Transaction represents a basic transaction
type Transaction struct {
	ID        string            `json:"id"`
	Timestamp int64             `json:"timestamp"`
	Sender    string            `json:"sender"`
	Recipient string            `json:"recipient"`
	Data      []byte            `json:"data"`
	Metadata  map[string]string `json:"metadata"`
	Signature []byte            `json:"signature,omitempty"`
}

// NewTransaction creates a new transaction
func NewTransaction(sender, recipient string, data []byte) *Transaction {
	tx := &Transaction{
		Timestamp: time.Now().Unix(),
		Sender:    sender,
		Recipient: recipient,
		Data:      data,
		Metadata:  make(map[string]string),
	}
	
	tx.ID = tx.CalculateHash()
	return tx
}

// CalculateHash generates a SHA256 hash of the transaction
func (tx *Transaction) CalculateHash() string {
	record := fmt.Sprintf("%d%s%s%s", tx.Timestamp, tx.Sender, tx.Recipient, tx.Data)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// Sign signs the transaction with a key pair
func (tx *Transaction) Sign(keyPair *crypto.KeyPair) error {
	// Clear any existing signature
	tx.Signature = nil
	
	// Serialize transaction data
	data, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %v", err)
	}
	
	// Sign data
	signature, err := keyPair.Sign(data)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}
	
	tx.Signature = signature
	return nil
}

// VerifySignature verifies the transaction signature
func (tx *Transaction) VerifySignature(publicKey *ecdsa.PublicKey) bool {
	if tx.Signature == nil {
		return false
	}
	
	// Get signature
	signature := tx.Signature
	
	// Clear signature for verification
	tx.Signature = nil
	data, err := tx.Serialize()
	if err != nil {
		return false
	}
	
	// Restore signature
	tx.Signature = signature
	
	// Verify signature
	return crypto.Verify(publicKey, data, signature)
}

// Serialize converts the transaction to JSON
func (tx *Transaction) Serialize() ([]byte, error) {
	return json.Marshal(tx)
}

// Deserialize converts JSON to a transaction
func Deserialize(data []byte) (*Transaction, error) {
	var tx Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// SetMetadata adds a key-value pair to the transaction's metadata
func (tx *Transaction) SetMetadata(key, value string) {
	tx.Metadata[key] = value
}

// GetMetadata retrieves a value from the transaction's metadata
func (tx *Transaction) GetMetadata(key string) (string, bool) {
	value, exists := tx.Metadata[key]
	return value, exists
}

// TransactionPool manages pending transactions
type TransactionPool struct {
	Transactions map[string]*Transaction
	mutex        sync.RWMutex
}

// NewTransactionPool creates a new transaction pool
func NewTransactionPool() *TransactionPool {
	return &TransactionPool{
		Transactions: make(map[string]*Transaction),
	}
}

// AddTransaction adds a transaction to the pool
func (tp *TransactionPool) AddTransaction(tx *Transaction) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	
	tp.Transactions[tx.ID] = tx
}

// GetTransaction retrieves a transaction from the pool
func (tp *TransactionPool) GetTransaction(id string) (*Transaction, bool) {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()
	
	tx, exists := tp.Transactions[id]
	return tx, exists
}

// RemoveTransaction removes a transaction from the pool
func (tp *TransactionPool) RemoveTransaction(id string) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	
	delete(tp.Transactions, id)
}

// GetPendingTransactions returns all pending transactions
func (tp *TransactionPool) GetPendingTransactions() []*Transaction {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()
	
	transactions := make([]*Transaction, 0, len(tp.Transactions))
	for _, tx := range tp.Transactions {
		transactions = append(transactions, tx)
	}
	
	return transactions
}

// Clear empties the transaction pool
func (tp *TransactionPool) Clear() {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	
	tp.Transactions = make(map[string]*Transaction)
}
