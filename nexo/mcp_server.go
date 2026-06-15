package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NexoVersion is the version of the Nexo MCP server
const NexoVersion = "1.0.0"

// NexoServer holds the MCP server and configuration
type NexoServer struct {
	server *mcp.Server
	nexoBin string
}

// NewNexoServer creates a new Nexo MCP server
func NewNexoServer() *NexoServer {
	nexoBin := os.Getenv("NEXO_BIN")
	if nexoBin == "" {
		nexoBin = "/usr/local/bin/cortex"
	}

	return &NexoServer{
		nexoBin: nexoBin,
	}
}

// runNexo executes a nexo command and returns output
func (s *NexoServer) runNexo(cmd string, args ...string) (string, error) {
	fullCmd := append([]string{cmd}, args...)
	result, err := exec.Command(s.nexoBin, fullCmd...).CombinedOutput()
	if err != nil {
		return string(result), fmt.Errorf("nexo error: %w\n%s", err, result)
	}
	return string(result), nil
}

// SetupServer configures the MCP server with all tools
func (s *NexoServer) SetupServer() *mcp.Server {
	s.server = mcp.NewServer(
		&mcp.Implementation{
			Name:    "nexo-memory",
			Version: NexoVersion,
		},
		nil,
	)

	// Register tools
	s.registerTools()

	return s.server
}

// registerTools registers all Nexo MCP tools
func (s *NexoServer) registerTools() {
	// Handoff tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "handoff",
			Description: "Wake up Nexo and load full context (identity, memory, preferences)",
		},
		s.handleHandoff,
	)

	// Recall tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "recall",
			Description: "Recall associative memory context for a query",
		},
		s.handleRecall,
	)

	// Store tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "store",
			Description: "Store text in memory as an episode",
		},
		s.handleStore,
	)

	// Reflect tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "reflect",
			Description: "Create a reflection linked to current context",
		},
		s.handleReflect,
	)

	// Journal tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "journal",
			Description: "Save session journal and consolidate PWS preferences",
		},
		s.handleJournal,
	)

	// Prefs tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "prefs",
			Description: "Show user preferences (PWS)",
		},
		s.handlePrefs,
	)

	// Stats tool
	mcp.AddTool(s.server,
		&mcp.Tool{
			Name:        "stats",
			Description: "Show memory graph statistics",
		},
		s.handleStats,
	)
}

// Tool input types
type RecallInput struct {
	Query  string `json:"query"`
	Brief  bool   `json:"brief,omitempty"`
}

type StoreInput struct {
	Text        string  `json:"text"`
	Title       string  `json:"title,omitempty"`
	Importance  float64 `json:"importance,omitempty"`
}

type ReflectInput struct {
	Text string `json:"text"`
}

// Tool handlers
func (s *NexoServer) handleHandoff(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := s.runNexo("handoff")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handleRecall(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input RecallInput
	if err := req.Params.Arguments.Unmarshal(&input); err != nil {
		return mcp.NewToolResultError("invalid input: " + err.Error()), nil
	}

	cmd := "recall"
	if input.Brief {
		cmd = "recall-brief"
	}

	output, err := s.runNexo(cmd, input.Query)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handleStore(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input StoreInput
	if err := req.Params.Arguments.Unmarshal(&input); err != nil {
		return mcp.NewToolResultError("invalid input: " + err.Error()), nil
	}

	args := []string{input.Text}
	if input.Title != "" {
		args = append(args, "--title", input.Title)
	}
	if input.Importance > 0 {
		args = append(args, "--importance", fmt.Sprintf("%.2f", input.Importance))
	}

	output, err := s.runNexo("store", args...)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handleReflect(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input ReflectInput
	if err := req.Params.Arguments.Unmarshal(&input); err != nil {
		return mcp.NewToolResultError("invalid input: " + err.Error()), nil
	}

	output, err := s.runNexo("reflect", input.Text)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handleJournal(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := s.runNexo("journal")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handlePrefs(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := s.runNexo("prefs")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func (s *NexoServer) handleStats(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := s.runNexo("stats")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func main() {
	server := NewNexoServer()
	mcpServer := server.SetupServer()

	// Run over stdio
	if err := mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "Error running server: %v\n", err)
		os.Exit(1)
	}
}
