package integration

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EventType defines the type of blockchain event
type EventType string

// Define event types
const (
	EventNewBlock         EventType = "new_block"
	EventNewTransaction   EventType = "new_transaction"
	EventValidatorUpdate  EventType = "validator_update"
	EventBridgeOperation  EventType = "bridge_operation"
	EventConsensusMessage EventType = "consensus_message"
	EventAIDecision       EventType = "ai_decision"
	EventSystemAlert      EventType = "system_alert"
)

// Event represents a blockchain event that will be published to subscribers
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	PlaneID   string                 `json:"plane_id"`
	Data      map[string]interface{} `json:"data"`
}

// Subscription represents a subscription to a specific event type
type Subscription struct {
	ID        string
	EventType EventType
	PlaneID   string
	Channel   chan Event
}

// PubSub represents a publish-subscribe system
type PubSub struct {
	subscriptions map[string][]Subscription
	mu            sync.RWMutex
}

// NewPubSub creates a new publish-subscribe system
func NewPubSub() *PubSub {
	return &PubSub{
		subscriptions: make(map[string][]Subscription),
	}
}

// Subscribe adds a new subscription to the specified event type
func (ps *PubSub) Subscribe(eventType EventType, planeID string) (*Subscription, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Create a unique subscription ID
	subscriptionID := fmt.Sprintf("%s_%s_%d", eventType, planeID, time.Now().UnixNano())

	// Create a new subscription
	subscription := Subscription{
		ID:        subscriptionID,
		EventType: eventType,
		PlaneID:   planeID,
		Channel:   make(chan Event, 100), // Buffer up to 100 events
	}

	// Create the key for this subscription
	key := getSubscriptionKey(eventType, planeID)

	// Add to subscriptions map
	ps.subscriptions[key] = append(ps.subscriptions[key], subscription)

	return &subscription, nil
}

// Unsubscribe removes a subscription
func (ps *PubSub) Unsubscribe(subscription *Subscription) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Get the key for this subscription
	key := getSubscriptionKey(subscription.EventType, subscription.PlaneID)

	// Find and remove the subscription
	subscriptions, exists := ps.subscriptions[key]
	if !exists {
		return fmt.Errorf("no subscriptions found for %s", key)
	}

	for i, sub := range subscriptions {
		if sub.ID == subscription.ID {
			// Remove subscription by swapping with the last element and then truncating
			lastIndex := len(subscriptions) - 1
			subscriptions[i] = subscriptions[lastIndex]
			ps.subscriptions[key] = subscriptions[:lastIndex]

			// Close the channel
			close(sub.Channel)
			return nil
		}
	}

	return fmt.Errorf("subscription not found: %s", subscription.ID)
}

// Publish publishes an event to all subscribers of that event type
func (ps *PubSub) Publish(event Event) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Get the key for this event
	key := getSubscriptionKey(event.Type, event.PlaneID)

	// Get subscribers for this event type and plane ID
	subscribers, exists := ps.subscriptions[key]
	if !exists || len(subscribers) == 0 {
		// No subscribers, just return
		return nil
	}

	// Send the event to all subscribers (non-blocking)
	for _, subscriber := range subscribers {
		select {
		case subscriber.Channel <- event:
			// Event sent successfully
		default:
			// Channel is full, skip this subscriber
			fmt.Printf("Warning: Channel full for subscriber %s\n", subscriber.ID)
		}
	}

	// Also send to wildcard subscribers (those who want all events from this plane)
	wildcardKey := getSubscriptionKey("*", event.PlaneID)
	wildcardSubscribers, exists := ps.subscriptions[wildcardKey]
	if exists && len(wildcardSubscribers) > 0 {
		for _, subscriber := range wildcardSubscribers {
			select {
			case subscriber.Channel <- event:
				// Event sent successfully
			default:
				// Channel is full, skip this subscriber
				fmt.Printf("Warning: Channel full for wildcard subscriber %s\n", subscriber.ID)
			}
		}
	}

	return nil
}

// getSubscriptionKey generates a unique key for a subscription based on event type and plane ID
func getSubscriptionKey(eventType EventType, planeID string) string {
	return fmt.Sprintf("%s:%s", eventType, planeID)
}

// SerializeEvent converts an event to JSON
func SerializeEvent(event Event) ([]byte, error) {
	return json.Marshal(event)
}

// DeserializeEvent converts JSON to an event
func DeserializeEvent(data []byte) (Event, error) {
	var event Event
	err := json.Unmarshal(data, &event)
	return event, err
}

// CreateBlockEvent creates an event for a new block
func CreateBlockEvent(planeID string, blockData map[string]interface{}) Event {
	return Event{
		Type:      EventNewBlock,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      blockData,
	}
}

// CreateTransactionEvent creates an event for a new transaction
func CreateTransactionEvent(planeID string, txData map[string]interface{}) Event {
	return Event{
		Type:      EventNewTransaction,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      txData,
	}
}

// CreateValidatorEvent creates an event for validator updates
func CreateValidatorEvent(planeID string, validatorData map[string]interface{}) Event {
	return Event{
		Type:      EventValidatorUpdate,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      validatorData,
	}
}

// CreateBridgeEvent creates an event for bridge operations
func CreateBridgeEvent(planeID string, bridgeData map[string]interface{}) Event {
	return Event{
		Type:      EventBridgeOperation,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      bridgeData,
	}
}

// CreateConsensusEvent creates an event for consensus messages
func CreateConsensusEvent(planeID string, consensusData map[string]interface{}) Event {
	return Event{
		Type:      EventConsensusMessage,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      consensusData,
	}
}

// CreateAIDecisionEvent creates an event for AI decisions
func CreateAIDecisionEvent(planeID string, aiData map[string]interface{}) Event {
	return Event{
		Type:      EventAIDecision,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      aiData,
	}
}

// CreateSystemAlertEvent creates an event for system alerts
func CreateSystemAlertEvent(planeID string, alertData map[string]interface{}) Event {
	return Event{
		Type:      EventSystemAlert,
		Timestamp: time.Now().Unix(),
		PlaneID:   planeID,
		Data:      alertData,
	}
}