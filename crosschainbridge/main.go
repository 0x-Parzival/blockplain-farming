package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Define two "chains"
type Chain string

const (
	ChainA Chain = "ChainA"
	ChainB Chain = "ChainB"
)

// User represents the user and their balances on both chains.
type User struct {
	ID       string             `json:"id"`
	Balances map[Chain]float64  `json:"balances"`
}

var (
	users = make(map[string]*User)
	mutex = &sync.Mutex{}
)

// getOrCreateUser ensures a user exists in the system.
func getOrCreateUser(id string) *User {
	user, exists := users[id]
	if !exists {
		user = &User{
			ID:       id,
			Balances: map[Chain]float64{ChainA: 1000, ChainB: 0}, // start with 1000 tokens on ChainA
		}
		users[id] = user
	}
	return user
}

// bridgeTokens simulates bridging tokens from ChainA to ChainB.
func bridgeTokens(id string, amount float64) string {
	mutex.Lock()
	user := getOrCreateUser(id)

	// Check if the user has enough balance on ChainA
	if user.Balances[ChainA] < amount {
		mutex.Unlock()
		return "❌ Insufficient balance on ChainA"
	}

	// Deduct tokens from ChainA and simulate bridging
	user.Balances[ChainA] -= amount
	mutex.Unlock()

	// Simulate delay and confirmation to ChainB
	go func() {
		// Simulate network delay for bridging
		time.Sleep(time.Duration(rand.Intn(3)+2) * time.Second)

		// Lock to update ChainB
		mutex.Lock()
		user.Balances[ChainB] += amount
		mutex.Unlock()

		// Show confirmation in terminal
		fmt.Printf("✅ Bridged %v tokens from %v to %v for user %s\n", amount, ChainA, ChainB, id)
	}()

	return "🔁 Bridging initiated. Please wait for confirmation..."
}

// displayUser shows the current balance of a user across both chains.
func displayUser(id string) string {
	mutex.Lock()
	defer mutex.Unlock()
	user := getOrCreateUser(id)

	return fmt.Sprintf("User: %s\nBalance on %s: %.2f\nBalance on %s: %.2f\n", id, ChainA, user.Balances[ChainA], ChainB, user.Balances[ChainB])
}

// Simulate running the bridge system
func main() {
	var userID string
	var amount float64
	var action string

	// Initial greeting message
	fmt.Println("🌉 Welcome to the Cross-Chain Bridge Converter!")
	fmt.Println("You can perform the following actions:")
	fmt.Println("1. Deposit Tokens into ChainA")
	fmt.Println("2. Bridge Tokens from ChainA to ChainB")
	fmt.Println("3. Check your user balance")
	fmt.Println("4. Exit")
	
	// Main interaction loop
	for {
		fmt.Print("\nEnter your action (1/2/3/4): ")
		fmt.Scanln(&action)

		switch action {
		case "1":
			fmt.Print("Enter your User ID: ")
			fmt.Scanln(&userID)
			fmt.Print("Enter amount to deposit into ChainA: ")
			fmt.Scanln(&amount)
			// Simulate deposit
			fmt.Printf("Depositing %.2f tokens into ChainA for user %s...\n", amount, userID)
			getOrCreateUser(userID)
			fmt.Printf("✔️ User %s now has %.2f tokens on ChainA\n", userID, amount)

		case "2":
			fmt.Print("Enter your User ID: ")
			fmt.Scanln(&userID)
			fmt.Print("Enter amount to bridge from ChainA to ChainB: ")
			fmt.Scanln(&amount)
			// Simulate bridging
			result := bridgeTokens(userID, amount)
			fmt.Println(result)

		case "3":
			fmt.Print("Enter your User ID to check balance: ")
			fmt.Scanln(&userID)
			// Display user balance
			result := displayUser(userID)
			fmt.Println(result)

		case "4":
			fmt.Println("👋 Exiting Cross-Chain Bridge Simulator. Goodbye!")
			return

		default:
			fmt.Println("❌ Invalid action. Please choose 1, 2, 3, or 4.")
		}
	}
}
