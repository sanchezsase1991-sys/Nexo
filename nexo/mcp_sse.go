package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// SSE transport for MCP - allows persistent connection via HTTP

type SSEServer struct {
	clients map[chan []byte]bool
	mu      sync.Mutex
}

func NewSSEServer() *SSEServer {
	return &SSEServer{
		clients: make(map[chan []byte]bool),
	}
}

func (s *SSEServer) addClient(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[ch] = true
}

func (s *SSEServer) removeClient(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, ch)
}

// handleSSE handles the SSE endpoint - establishes event stream
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := make(chan []byte, 100)
	s.addClient(ch)
	defer s.removeClient(ch)

	// Send endpoint event so client knows where to POST
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp/message?sessionId=%s\n\n", sessionID)
	flusher.Flush()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleMessage handles the POST endpoint for JSON-RPC messages
func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process the request
	result := handleMCPOperation(req)

	// Send response
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
	data, _ := json.Marshal(resp)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleMCPOperation processes an MCP operation and returns the result
func handleMCPOperation(req JSONRPCRequest) interface{} {
	cfg := &DefaultConfig
	db, err := OpenDB(cfg)
	if err != nil {
		return map[string]interface{}{
			"code":    -32603,
			"message": fmt.Sprintf("Error opening DB: %v", err),
		}
	}
	defer db.Close()

	switch req.Method {
	case "initialize":
		return InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{"tools": map[string]interface{}{}},
			ServerInfo:       ServerInfo{Name: MCPName, Version: MCPVersion},
		}

	case "notifications/initialized":
		return nil

	case "tools/list":
		return map[string]interface{}{
			"tools": getMCPTools(),
		}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if paramsData, ok := req.Params.(map[string]interface{}); ok {
			params.Name, _ = paramsData["name"].(string)
			if args, ok := paramsData["arguments"].(map[string]interface{}); ok {
				params.Arguments = args
			}
		}
		return handleMCPCall(params.Name, params.Arguments)

	default:
		return map[string]interface{}{
			"code":    -32601,
			"message": "Method not found: " + req.Method,
		}
	}
}

// RunSSE starts the MCP server in SSE mode (HTTP)
func RunSSE(addr string) {
	server := NewSSEServer()

	http.HandleFunc("/sse", server.handleSSE)
	http.HandleFunc("/mcp/message", server.handleMessage)

	// Also handle direct message endpoint (some clients use this)
	http.HandleFunc("/message", server.handleMessage)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","name":"nexo-memory","version":"5.0.0","transport":"sse"}`))
	})

	fmt.Fprintf(os.Stderr, "🧠 Nexo MCP Server (SSE) listening on %s\n", addr)
	fmt.Fprintf(os.Stderr, "   SSE endpoint: %s/sse\n", addr)
	fmt.Fprintf(os.Stderr, "   Message endpoint: %s/message\n", addr)
	fmt.Fprintf(os.Stderr, "   Health: %s/health\n", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
