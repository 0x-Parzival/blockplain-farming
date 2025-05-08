// blockplain/cli/cli.go

package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/yourusername/blockplain/block"
	"github.com/yourusername/blockplain/config"
	"github.com/yourusername/blockplain/crypto"
	"github.com/yourusername/blockplain/transaction"
)

// CLI represents the command line interface
type CLI struct {
	ConfigPath string
	APIAddress string
}

// New creates a new CLI instance
func New(configPath string) (*CLI, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}
	
	apiAddress := fmt.Sprintf("http://localhost:%d", cfg.APIPort)
	
	return &CLI{
		ConfigPath: configPath,
		APIAddress: apiAddress,
	}, nil
}

// PrintHelp prints help information
func (cli *CLI) PrintHelp() {
	fmt.Println("BlockPlain - Simple Blockchain CLI")
	fmt.Println("Usage:")
	fmt.Println("  chain                 - Show blockchain info")
	fmt.Println("  blocks                - List all blocks")
	fmt.Println("  block <index|hash>    - Show block details")
	fmt.Println("  add <data>            - Add a new block with data")
	fmt.Println("  validate              - Validate the blockchain")
	fmt.Println("  peers                 - List all peers")
	fmt.Println("  addpeer <address>     - Add a new peer")
	fmt.Println("  tx create <sender> <recipient> <data> - Create a new transaction")
	fmt.Println("  tx sign <tx_file> <private_key>      - Sign a transaction")
	fmt.Println("  tx send <tx_file>                    - Send a transaction")
	fmt.Println("  keygen <output_dir>   - Generate a new key pair")
	fmt.Println("  help                  - Show this help")
}

// Run executes a CLI command
func (cli *CLI) Run(args []string) error {
	if len(args) < 1 {
		cli.PrintHelp()
		return nil
	}
	
	command := args[0]
	switch command {
	case "chain":
		return cli.showChainInfo()
	case "blocks":
		return cli.listBlocks()
	case "block":
		if len(args) < 2 {
			return fmt.Errorf("block command requires an index or hash")
		}
		return cli.showBlock(args[1])
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("add command requires data")
		}
		return cli.addBlock(strings.Join(args[1:], " "))
	case "validate":
		return cli.validateChain()
	case "peers":
		return cli.listPeers()
	case "addpeer":
		if len(args) < 2 {
			return fmt.Errorf("addpeer command requires an address")
		}
		return cli.addPeer(args[1])
	case "tx":
		if len(args) < 2 {
			return fmt.Errorf("tx command requires a subcommand")
		}
		switch args[1] {
		case "create":
			if len(args) < 5 {
				return fmt.Errorf("tx create requires sender, recipient, and data")
			}
			return cli.createTransaction(args[2], args[3], args[4:])
		case "sign":
			if len(args) < 4 {
				return fmt.Errorf("tx sign requires a transaction file and private key")
			}
			return cli.signTransaction(args[2], args[3])
		case "send":
			if len(args) < 3 {
				return fmt.Errorf("tx send requires a transaction file")
			}
			return cli.sendTransaction(args[2])
		default:
			return fmt.Errorf("unknown tx subcommand: %s", args[1])
		}
	case "keygen":
		if len(args) < 2 {
			return fmt.Errorf("keygen command requires an output directory")
		}
		return cli.generateKeyPair(args[1])
	case "help":
		cli.PrintHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// showChainInfo shows information about the blockchain
func (cli *CLI) showChainInfo() error {
	resp, err := http.Get(fmt.Sprintf("%s/block/latest", cli.APIAddress))
	if err != nil {
		return fmt.Errorf("failed to get latest block: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	var latestBlock block.Block
	if err := json.Unmarshal(body, &latestBlock); err != nil {
		return fmt.Errorf("failed to parse latest block: %v", err)
	}
	
	fmt.Println("Blockchain Info:")
	fmt.Printf("  Latest Block: %d\n", latestBlock.Index)
	fmt.Printf("  Latest Hash: %s\n", latestBlock.Hash)
	fmt.Printf("  Timestamp: %d\n", latestBlock.Timestamp)
	
	return nil
}

// listBlocks lists all blocks in the chain
func (cli *CLI) listBlocks() error {
	resp, err := http.Get(fmt.Sprintf("%s/blocks", cli.APIAddress))
	if err != nil {
		return fmt.Errorf("failed to get blocks: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	var blocks []*block.Block
	if err := json.Unmarshal(body, &blocks); err != nil {
		return fmt.Errorf("failed to parse blocks: %v", err)
	}
	
	fmt.Printf("Blockchain (%d blocks):\n", len(blocks))
	for _, block := range blocks {
		fmt.Printf("  Block %d: %s\n", block.Index, block.Hash[:8])
	}
	
	return nil
}

// showBlock shows details of a specific block
func (cli *CLI) showBlock(indexOrHash string) error {
	var url string
	
	// Check if input is a number (index) or hash
	if _, err := strconv.ParseUint(indexOrHash, 10, 64); err == nil {
		url = fmt.Sprintf("%s/block?index=%s", cli.APIAddress, indexOrHash)
	} else {
		url = fmt.Sprintf("%s/block?hash=%s", cli.APIAddress, indexOrHash)
	}
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get block: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	var b block.Block
	if err := json.Unmarshal(body, &b); err != nil {
		return fmt.Errorf("failed to parse block: %v", err)
	}
	
	fmt.Println("Block Details:")
	fmt.Printf("  Index: %d\n", b.Index)
	fmt.Printf("  Hash: %s\n", b.Hash)
	fmt.Printf("  Previous Hash: %s\n", b.PreviousHash)
	fmt.Printf("  Timestamp: %d\n", b.Timestamp)
	fmt.Printf("  Data: %s\n", string(b.Data))
	
	fmt.Println("  Metadata:")
	for k, v := range b.Metadata {
		fmt.Printf("    %s: %s\n", k, v)
	}
	
	return nil
}

// addBlock adds a new block to the chain
func (cli *CLI) addBlock(data string) error {
	resp, err := http.Post(
		fmt.Sprintf("%s/block/add", cli.APIAddress),
		"text/plain",
		strings.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to add block: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	var b block.Block
	if err := json.Unmarshal(body, &b); err != nil {
		return fmt.Errorf("failed to parse block: %v", err)
	}
	
	fmt.Printf("Block added successfully (index: %d, hash: %s)\n", b.Index, b.Hash)
	return nil
}

// validateChain validates the blockchain
func (cli *CLI) validateChain() error {
	resp, err := http.Get(fmt.Sprintf("%s/chain/validate", cli.APIAddress))
	if err != nil {
		return fmt.Errorf("failed to validate chain: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	var result struct {
		Valid bool `json:"valid"`
	}
	
	if err := json.U