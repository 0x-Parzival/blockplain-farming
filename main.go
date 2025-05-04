package main
//deposit chainA 0x1a3f8c2d5e7b1a9f0c4d6e8b3a5c7d9e1f2a4b6 100
import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Blockchain struct {
	Name      string
	HexPrefix string
	Accounts  map[string]*Account
	mu        sync.Mutex
}

type Account struct {
	Address string
	Balance float64
	Staked  float64
	Yield   float64
}

var (
	blockchains = make(map[string]*Blockchain)
	running     = true
)

func main() {
	// Initialize two blockchains with different hex prefixes
	blockchains["chainA"] = &Blockchain{
		Name:      "Chain A",
		HexPrefix: "0x1a",
		Accounts:  make(map[string]*Account),
	}
	blockchains["chainB"] = &Blockchain{
		Name:      "Chain B",
		HexPrefix: "0x2b",
		Accounts:  make(map[string]*Account),
	}

	// Start yield generation goroutine
	go generateYield()

	fmt.Println("Blockchain CLI Interface")
	fmt.Println("Available chains:", getChainNames())
	printHelp()

	reader := bufio.NewReader(os.Stdin)

	for running {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		parts := strings.Split(input, " ")

		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch command {
		case "create":
			if len(args) != 1 {
				fmt.Println("Usage: create [chainName]")
				continue
			}
			createAccount(args[0])
		case "deposit":
			if len(args) != 3 {
				fmt.Println("Usage: deposit [chainName] [address] [amount]")
				continue
			}
			deposit(args[0], args[1], args[2])
		case "stake":
			if len(args) != 3 {
				fmt.Println("Usage: stake [chainName] [address] [amount]")
				continue
			}
			stake(args[0], args[1], args[2])
		case "withdraw":
			if len(args) != 2 {
				fmt.Println("Usage: withdraw [chainName] [address]")
				continue
			}
			withdraw(args[0], args[1])
		case "balance":
			if len(args) != 2 {
				fmt.Println("Usage: balance [chainName] [address]")
				continue
			}
			getBalance(args[0], args[1])
		case "transfer":
			if len(args) != 4 {
				fmt.Println("Usage: transfer [fromChain] [toChain] [address] [amount]")
				continue
			}
			transfer(args[0], args[1], args[2], args[3])
		case "chains":
			fmt.Println("Available chains:", getChainNames())
		case "help":
			printHelp()
		case "exit", "quit":
			running = false
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
	}
}

func generateYield() {
	for running {
		time.Sleep(10 * time.Second)
		for _, chain := range blockchains {
			chain.mu.Lock()
			for _, account := range chain.Accounts {
				if account.Staked > 0 {
					// Generate random yield between 0.1% and 5%
					yieldPercent := 0.1 + (4.9 * randFloat())
					yield := account.Staked * yieldPercent / 100
					account.Yield += yield
				}
			}
			chain.mu.Unlock()
		}
	}
}

func randFloat() float64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return float64(n.Int64()) / 1000
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  create [chainName]          - Create new account on specified chain")
	fmt.Println("  deposit [chain] [addr] [amt] - Deposit funds to account")
	fmt.Println("  stake [chain] [addr] [amt]   - Stake tokens to earn yield")
	fmt.Println("  withdraw [chain] [addr]      - Claim earned yield")
	fmt.Println("  balance [chain] [addr]       - Check account balance")
	fmt.Println("  transfer [from] [to] [addr] [amt] - Transfer between chains")
	fmt.Println("  chains                      - List available chains")
	fmt.Println("  help                        - Show this help message")
	fmt.Println("  exit                        - Quit the program\n")
}

func createAccount(chainName string) {
	chain, exists := blockchains[chainName]
	if !exists {
		fmt.Println("Chain not found. Available chains:", getChainNames())
		return
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	// Generate random address with chain's hex prefix
	addr := generateAddress(chain.HexPrefix)
	chain.Accounts[addr] = &Account{
		Address: addr,
		Balance: 0,
		Staked:  0,
		Yield:   0,
	}

	fmt.Printf("Created new account on %s: %s\n", chain.Name, addr)
}

func generateAddress(prefix string) string {
	bytes := make([]byte, 20)
	rand.Read(bytes)
	hash := sha256.Sum256(bytes)
	return prefix + hex.EncodeToString(hash[:])[:40]
}

func deposit(chainName, address, amountStr string) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		fmt.Println("Invalid amount. Must be a positive number.")
		return
	}

	chain, exists := blockchains[chainName]
	if !exists {
		fmt.Println("Chain not found. Available chains:", getChainNames())
		return
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	account, exists := chain.Accounts[address]
	if !exists {
		fmt.Println("Account not found on this chain")
		return
	}

	account.Balance += amount
	fmt.Printf("Deposited %.2f to %s on %s. New balance: %.2f\n", 
		amount, address, chain.Name, account.Balance)
}

func stake(chainName, address, amountStr string) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		fmt.Println("Invalid amount. Must be a positive number.")
		return
	}

	chain, exists := blockchains[chainName]
	if !exists {
		fmt.Println("Chain not found. Available chains:", getChainNames())
		return
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	account, exists := chain.Accounts[address]
	if !exists {
		fmt.Println("Account not found on this chain")
		return
	}

	if account.Balance < amount {
		fmt.Println("Insufficient balance")
		return
	}

	account.Balance -= amount
	account.Staked += amount
	fmt.Printf("Staked %.2f from %s on %s. Staked: %.2f, Available: %.2f\n", 
		amount, address, chain.Name, account.Staked, account.Balance)
}

func withdraw(chainName, address string) {
	chain, exists := blockchains[chainName]
	if !exists {
		fmt.Println("Chain not found. Available chains:", getChainNames())
		return
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	account, exists := chain.Accounts[address]
	if !exists {
		fmt.Println("Account not found on this chain")
		return
	}

	if account.Yield <= 0 {
		fmt.Println("No yield to withdraw")
		return
	}

	yield := account.Yield
	account.Balance += yield
	account.Yield = 0
	fmt.Printf("Withdrawn %.2f yield from %s on %s. New balance: %.2f\n", 
		yield, address, chain.Name, account.Balance)
}

func getBalance(chainName, address string) {
	chain, exists := blockchains[chainName]
	if !exists {
		fmt.Println("Chain not found. Available chains:", getChainNames())
		return
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	account, exists := chain.Accounts[address]
	if !exists {
		fmt.Println("Account not found on this chain")
		return
	}

	fmt.Printf("\nAccount %s on %s\n", address, chain.Name)
	fmt.Println("-------------------")
	fmt.Printf("Available balance: %.2f\n", account.Balance)
	fmt.Printf("Staked amount:    %.2f\n", account.Staked)
	fmt.Printf("Pending yield:    %.2f\n\n", account.Yield)
}

func transfer(fromChain, toChain, address, amountStr string) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		fmt.Println("Invalid amount. Must be a positive number.")
		return
	}

	srcChain, exists := blockchains[fromChain]
	if !exists {
		fmt.Println("Source chain not found")
		return
	}

	destChain, exists := blockchains[toChain]
	if !exists {
		fmt.Println("Destination chain not found")
		return
	}

	srcChain.mu.Lock()
	defer srcChain.mu.Unlock()

	destChain.mu.Lock()
	defer destChain.mu.Unlock()

	// Check if account exists on source chain
	srcAccount, exists := srcChain.Accounts[address]
	if !exists {
		fmt.Println("Account not found on source chain")
		return
	}

	if srcAccount.Balance < amount {
		fmt.Println("Insufficient balance for transfer")
		return
	}

	// Check if account exists on destination chain, create if not
	destAccount, exists := destChain.Accounts[address]
	if !exists {
		destAccount = &Account{
			Address: address,
			Balance: 0,
			Staked:  0,
			Yield:   0,
		}
		destChain.Accounts[address] = destAccount
	}

	// Perform the transfer
	srcAccount.Balance -= amount
	destAccount.Balance += amount

	fmt.Printf("Transferred %.2f from %s to %s for account %s\n",
		amount, srcChain.Name, destChain.Name, address)
}

func getChainNames() []string {
	names := make([]string, 0, len(blockchains))
	for name := range blockchains {
		names = append(names, name)
	}
	return names
}