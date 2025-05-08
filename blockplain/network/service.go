// blockplain/network/service.go
package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/blockplain/config"
	"github.com/blockplain/core"
	"google.golang.org/grpc"
	pb "github.com/blockplain/proto"
)

// Service handles network communication between nodes
type Service struct {
	config       config.NetworkConfig
	planes       map[string]*core.Plane
	communicator *core.CrossPlaneManager
	server       *grpc.Server
	listener     net.Listener
	peers        map[string]*peer
	peersMutex   sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// peer represents a connection to another node
type peer struct {
	address string
	client  pb.BlockplainClient
	conn    *grpc.ClientConn
}

// NewService creates a new network service
func NewService(cfg config.NetworkConfig, planes map[string]*core.Plane, communicator *core.CrossPlaneManager) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Service{
		config:       cfg,
		planes:       planes,
		communicator: communicator,
		peers:        make(map[string]*peer),
		peersMutex:   sync.RWMutex{},
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start starts the network service
func (s *Service) Start() error {
	// Start gRPC server
	if err := s.startServer(); err != nil {
		return err
	}
	
	// Connect to peers
	if err := s.connectToPeers(); err != nil {
		return err
	}
	
	// Start sync process
	go s.startSyncProcess()
	
	return nil
}

// Stop stops the network service
func (s *Service) Stop() {
	s.cancel()
	
	// Stop gRPC server
	if s.server != nil {
		s.server.GracefulStop()
	}
	
	// Close peer connections
	s.peersMutex.Lock()
	defer s.peersMutex.Unlock()
	
	for _, p := range s.peers {
		p.conn.Close()
	}
}

// startServer starts the gRPC server
func (s *Service) startServer() error {
	lis, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	
	s.listener = lis
	s.server = grpc.NewServer()
	
	// Register service
	pb.RegisterBlockplainServer(s.server, &blockplainServer{
		service: s,
	})
	
	// Start server in a goroutine
	go func() {
		if err := s.server.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()
	
	log.Printf("Network service listening on %s", s.config.ListenAddress)
	return nil
}

// connectToPeers connects to all configured peers
func (s *Service) connectToPeers() error {
	for _, peerAddr := range s.config.Peers {
		if err := s.connectToPeer(peerAddr); err != nil {
			log.Printf("Failed to connect to peer %s: %v", peerAddr, err)
		}
	}
	return nil
}

// connectToPeer connects to a single peer
func (s *Service) connectToPeer(address string) error {
	// Check if already connected
	s.peersMutex.RLock()
	if _, exists := s.peers[address]; exists {
		s.peersMutex.RUnlock()
		return nil
	}
	s.peersMutex.RUnlock()
	
	// Connect to peer
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %v", err)
	}
	
	client := pb.NewBlockplainClient(conn)
	
	// Add peer to list
	s.peersMutex.Lock()
	s.peers[address] = &peer{
		address: address,
		client:  client,
		conn:    conn,
	}
	s.peersMutex.Unlock()
	
	log.Printf("Connected to peer %s", address)
	return nil
}

// startSyncProcess periodically syncs with peers
func (s *Service) startSyncProcess() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Sync with peers
			s.syncWithPeers()
		}
	}
}

// syncWithPeers exchanges state information with all peers
func (s *Service) syncWithPeers() {
	s.peersMutex.RLock()
	defer s.peersMutex.RUnlock()
	
	for addr, p := range s.peers {
		// Get state from peer
		ctx, cancel := context.WithTimeout(s.ctx, s.config.RequestTimeout)
		resp, err := p.client.GetState(ctx, &pb.GetStateRequest{})
		cancel()
		
		if err != nil {
			log.Printf("Failed to get state from peer %s: %v", addr, err)
			continue
		}
		
		// Process state update
		s.processStateUpdate(resp)
	}
}

// processStateUpdate processes state updates from peers
func (s *Service) processStateUpdate(state *pb.GetStateResponse) {
	// Process state for each plane
	for _, planeState := range state.PlaneStates {
		// Get local plane
		localPlane, exists := s.planes[planeState.PlaneId]
		if !exists {
			// Ignore updates for planes we don't have
			continue
		}
		
		// Check if we need to sync
		if planeState.Height > localPlane.CurrentHeight {
			// Request blocks we don't have
			s.syncMissingBlocks(planeState.PlaneId, localPlane.CurrentHeight, planeState.Height)
		}
	}
}

// syncMissingBlocks syncs missing blocks from peers
func (s *Service) syncMissingBlocks(planeID string, localHeight, remoteHeight uint64) {
	// Implementation would request missing blocks from peers
	// For simplicity, we'll just log the sync operation
	log.Printf("Syncing plane %s from height %d to %d", planeID, localHeight, remoteHeight)
}

// blockplainServer implements the gRPC server
type blockplainServer struct {
	pb.UnimplementedBlockplainServer
	service *Service
}

// GetState handles GetState RPC requests
func (s *blockplainServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.GetStateResponse, error) {
	// Get state of all planes
	planeStates := make([]*pb.PlaneState, 0)
	
	for id, plane := range s.service.planes {
		planeStates = append(planeStates, &pb.PlaneState{
			PlaneId: id,
			Height:  plane.CurrentHeight,
			Hash:    plane.LastHash,
		})
	}
	
	return &pb.GetStateResponse{
		PlaneStates: planeStates,
	}, nil
}

// GetBlock handles GetBlock RPC requests
func (s *blockplainServer) GetBlock(ctx context.Context, req *pb.GetBlockRequest) (*pb.GetBlockResponse, error) {
	// Get plane
	plane, exists := s.service.planes[req.PlaneId]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.PlaneId)
	}
	
	// Get block
	block, err := plane.GetBlock(req.Hash)
	if err != nil {
		return nil, err
	}
	
	// Convert to protobuf block
	pbBlock := &pb.Block{
		Height:    block.Height,
		Hash:      block.Hash,
		PrevHash:  block.PrevHash,
		PlaneId:   block.PlaneID,
		Timestamp: block.Timestamp,
		Validator: block.Validator,
		Signature: block.Signature,
	}
	
	// Add transactions
	for _, tx := range block.Transactions {
		pbBlock.Transactions = append(pbBlock.Transactions, &pb.Transaction{
			TxId:        tx.TxID,
			From:        tx.From,
			To:          tx.To,
			Amount:      tx.Amount,
			Data:        tx.Data,
			Signature:   tx.Signature,
			Timestamp:   tx.Timestamp,
			PlaneId:     tx.PlaneID,
			TargetPlane: tx.TargetPlane,
			GasPrice:    tx.GasPrice,
			GasLimit:    tx.GasLimit,
			Status:      tx.Status,
		})
	}
	
	// Add cross-references
	for _, ref := range block.CrossRefs {
		pbBlock.CrossRefs = append(pbBlock.CrossRefs, &pb.CrossPlaneRef{
			PlaneId:   ref.PlaneID,
			BlockHash: ref.BlockHash,
			Height:    ref.Height,
			Timestamp: ref.Timestamp,
		})
	}
	
	return &pb.GetBlockResponse{
		Block: pbBlock,
	}, nil
}

// SubmitTransaction handles SubmitTransaction RPC requests
func (s *blockplainServer) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	// Get plane
	plane, exists := s.service.planes[req.Transaction.PlaneId]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.Transaction.PlaneId)
	}
	
	// Convert to core transaction
	tx := core.Transaction{
		TxID:        req.Transaction.TxId,
		From:        req.Transaction.From,
		To:          req.Transaction.To,
		Amount:      req.Transaction.Amount,
		Data:        req.Transaction.Data,
		Signature:   req.Transaction.Signature,
		Timestamp:   req.Transaction.Timestamp,
		PlaneID:     req.Transaction.PlaneId,
		TargetPlane: req.Transaction.TargetPlane,
		GasPrice:    req.Transaction.GasPrice,
		GasLimit:    req.Transaction.GasLimit,
		Status:      req.Transaction.Status,
	}
	
	// Add transaction to plane
	if err := plane.AddTransaction(tx); err != nil {
		return nil, err
	}
	
	return &pb.SubmitTransactionResponse{
		Success: true,
		TxId:    tx.TxID,
	}, nil
}

// APIServer implements the JSON-RPC API server
type APIServer struct {
	config       config.APIConfig
	planes       map[string]*core.Plane
	communicator *core.CrossPlaneManager
	publisher    pubsub.Publisher
	server       *http.Server
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewAPIServer creates a new API server
func NewAPIServer(cfg config.APIConfig, planes map[string]*core.Plane, 
                 communicator *core.CrossPlaneManager, publisher pubsub.Publisher) (*APIServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &APIServer{
		config:       cfg,
		planes:       planes,
		communicator: communicator,
		publisher:    publisher,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// Start starts the API server
func (s *APIServer) Start() error {
	// Create router
	router := http.NewServeMux()
	
	// Register RPC handler
	router.HandleFunc("/rpc", s.handleRPC)
	
	// Register status endpoint
	router.HandleFunc("/status", s.handleStatus)
	
	// Create server
	s.server = &http.Server{
		Addr:    s.config.ListenAddress,
		Handler: router,
	}
	
	// Start server
	go func() {
		log.Printf("API server listening on %s", s.config.ListenAddress)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()
	
	return nil
}

// Stop stops the API server
func (s *APIServer) Stop() {
	s.cancel()
	
	// Create shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()
	
	// Shutdown server
	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}
}

// handleRPC handles JSON-RPC requests
func (s *APIServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse request
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      interface{}     `json:"id"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Process request
	var result interface{}
	var err error
	
	switch req.Method {
	case "getPlaneState":
		result, err = s.handleGetPlaneState(req.Params)
	case "getBlock":
		result, err = s.handleGetBlock(req.Params)
	case "getTransaction":
		result, err = s.handleGetTransaction(req.Params)
	case "submitTransaction":
		result, err = s.handleSubmitTransaction(req.Params)
	case "processCrossPlaneTransaction":
		result, err = s.handleProcessCrossPlaneTransaction(req.Params)
	case "applyAIDecision":
		result, err = s.handleApplyAIDecision(req.Params)
	default:
		err = fmt.Errorf("method not found: %s", req.Method)
	}
	
	// Prepare response
	var resp struct {
		JSONRPC string      `json:"jsonrpc"`
		Result  interface{} `json:"result,omitempty"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		ID interface{} `json:"id"`
	}
	
	resp.JSONRPC = "2.0"
	resp.ID = req.ID
	
	if err != nil {
		resp.Error = &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{
			Code:    -32000,
			Message: err.Error(),
		}
	} else {
		resp.Result = result
	}
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// handleStatus handles status requests
func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Get system status
	status := map[string]interface{}{
		"planes":       s.communicator.GetPlaneState(),
		"version":      "0.1.0",
		"uptime":       time.Now().Unix() - s.startTime.Unix(),
		"current_time": time.Now().Unix(),
	}
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Failed to encode status: %v", err)
	}
}

// handleGetPlaneState handles getPlaneState requests
func (s *APIServer) handleGetPlaneState(params json.RawMessage) (interface{}, error) {
	var req struct {
		PlaneID string `json:"plane_id"`
	}
	
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// If plane ID is empty, return state of all planes
	if req.PlaneID == "" {
		return s.communicator.GetPlaneState(), nil
	}
	
	// Get plane
	plane, exists := s.planes[req.PlaneID]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.PlaneID)
	}
	
	// Get plane state
	planeState := map[string]interface{}{
		"id":              req.PlaneID,
		"current_height":  plane.CurrentHeight,
		"last_hash":       plane.LastHash,
		"validator":       plane.CurrentValidator,
		"block_count":     len(plane.Blocks),
		"pending_txs":     len(plane.PendingTxs),
	}
	
	return planeState, nil
}

// handleGetBlock handles getBlock requests
func (s *APIServer) handleGetBlock(params json.RawMessage) (interface{}, error) {
	var req struct {
		PlaneID string `json:"plane_id"`
		Hash    string `json:"hash"`
		Height  uint64 `json:"height"`
	}
	
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// Get plane
	plane, exists := s.planes[req.PlaneID]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.PlaneID)
	}
	
	// Get block by hash or height
	var block *core.Block
	var err error
	
	if req.Hash != "" {
		block, err = plane.GetBlock(req.Hash)
	} else if req.Height > 0 {
		// Implementation would have GetBlockByHeight
		return nil, fmt.Errorf("getBlockByHeight not implemented")
	} else {
		return nil, fmt.Errorf("hash or height required")
	}
	
	if err != nil {
		return nil, err
	}
	
	return block, nil
}

// handleGetTransaction handles getTransaction requests
func (s *APIServer) handleGetTransaction(params json.RawMessage) (interface{}, error) {
	var req struct {
		PlaneID string `json:"plane_id"`
		TxID    string `json:"tx_id"`
	}
	
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// Get plane
	plane, exists := s.planes[req.PlaneID]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.PlaneID)
	}
	
	// Get transaction
	tx, err := plane.GetTransaction(req.TxID)
	if err != nil {
		return nil, err
	}
	
	return tx, nil
}

// handleSubmitTransaction handles submitTransaction requests
func (s *APIServer) handleSubmitTransaction(params json.RawMessage) (interface{}, error) {
	var tx core.Transaction
	
	if err := json.Unmarshal(params, &tx); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// Get plane
	plane, exists := s.planes[tx.PlaneID]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", tx.PlaneID)
	}
	
	// Add transaction
	if err := plane.AddTransaction(tx); err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"success": true,
		"tx_id":   tx.TxID,
	}, nil
}

// handleProcessCrossPlaneTransaction handles processCrossPlaneTransaction requests
func (s *APIServer) handleProcessCrossPlaneTransaction(params json.RawMessage) (interface{}, error) {
	var tx core.Transaction
	
	if err := json.Unmarshal(params, &tx); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// Forward transaction to target plane
	if err := s.communicator.ForwardTransaction(tx); err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"success": true,
		"tx_id":   tx.TxID,
	}, nil
}

// handleApplyAIDecision handles applyAIDecision requests
func (s *APIServer) handleApplyAIDecision(params json.RawMessage) (interface{}, error) {
	var req struct {
		PlaneID  string                 `json:"plane_id"`
		Decision string                 `json:"decision"`
		Data     map[string]interface{} `json:"data"`
	}
	
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %v", err)
	}
	
	// Get plane
	plane, exists := s.planes[req.PlaneID]
	if !exists {
		return nil, fmt.Errorf("plane not found: %s", req.PlaneID)
	}
	
	// Process AI decision
	// This would apply the AI decision to the blockchain
	// For now, just publish an event
	aiEvent := map[string]interface{}{
		"event_type": "ai_decision",
		"plane_id":   req.PlaneID,
		"decision":   req.Decision,
		"data":       req.Data,
		"timestamp":  time.Now().Unix(),
	}
	s.publisher.Publish("ai_decisions", aiEvent)
	
	return map[string]interface{}{
		"success":  true,
		"applied": true,
	}, nil
}