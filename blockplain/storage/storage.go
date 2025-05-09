// blockplain/storage/storage.go

package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"https://github.com/0x-Parzival/blockplain-farming/blockplain/block"
)

// Storage handles blockchain persistence
type Storage struct {
	dataDir string
	mutex   sync.RWMutex
}

// New creates a new storage instance
func New(dataDir string) (*Storage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}
	
	return &Storage{
		dataDir: dataDir,
	}, nil
}

// SaveBlock saves a block to disk
func (s *Storage) SaveBlock(b *block.Block) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	data, err := b.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize block: %v", err)
	}
	
	filename := filepath.Join(s.dataDir, fmt.Sprintf("block_%d.json", b.Index))
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write block file: %v", err)
	}
	
	return nil
}

// LoadBlock loads a block from disk by index
func (s *Storage) LoadBlock(index uint64) (*block.Block, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	filename := filepath.Join(s.dataDir, fmt.Sprintf("block_%d.json", index))
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read block file: %v", err)
	}
	
	b, err := block.Deserialize(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %v", err)
	}
	
	return b, nil
}

// SaveChainInfo saves metadata about the chain
func (s *Storage) SaveChainInfo(length uint64, latestHash string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	info := struct {
		Length     uint64 `json:"length"`
		LatestHash string `json:"latest_hash"`
	}{
		Length:     length,
		LatestHash: latestHash,
	}
	
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to serialize chain info: %v", err)
	}
	
	filename := filepath.Join(s.dataDir, "chain_info.json")
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write chain info file: %v", err)
	}
	
	return nil
}

// LoadChainInfo loads metadata about the chain
func (s *Storage) LoadChainInfo() (uint64, string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	filename := filepath.Join(s.dataDir, "chain_info.json")
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("failed to read chain info file: %v", err)
	}
	
	var info struct {
		Length     uint64 `json:"length"`
		LatestHash string `json:"latest_hash"`
	}
	
	if err := json.Unmarshal(data, &info); err != nil {
		return 0, "", fmt.Errorf("failed to parse chain info: %v", err)
	}
	
	return info.Length, info.LatestHash, nil
}

// ListBlockFiles returns a list of all block files
func (s *Storage) ListBlockFiles() ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	pattern := filepath.Join(s.dataDir, "block_*.json")
	return filepath.Glob(pattern)
}

// LoadAllBlocks loads all blocks from disk
func (s *Storage) LoadAllBlocks() ([]*block.Block, error) {
	files, err := s.ListBlockFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list block files: %v", err)
	}
	
	blocks := make([]*block.Block, 0, len(files))
	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read block file %s: %v", file, err)
		}
		
		b, err := block.Deserialize(data)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block from %s: %v", file, err)
		}
		
		blocks = append(blocks, b)
	}
	
	return blocks, nil
}
