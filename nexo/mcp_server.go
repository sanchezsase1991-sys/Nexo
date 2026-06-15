package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ServerName    = "nexo-memory"
	ServerVersion = "1.0.0"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    interface{} `json:"capabilities"`
	ServerInfo      ServerInfo  `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var nexoBin string

func init() {
	nexoBin = os.Getenv("NEXO_BIN")
	if nexoBin == "" {
		nexoBin = "/usr/local/bin/cortex"
	}
}

func runNexo(cmd string, args ...string) (string, error) {
	fullCmd := append([]string{cmd}, args...)
	result, err := exec.Command(nexoBin, fullCmd...).CombinedOutput()
	if err != nil {
		return string(result), fmt.Errorf("nexo error: %w", err)
	}
	return string(result), nil
}

func getTools() []Tool {
	return []Tool{
		{
			Name:        "handoff",
			Description: "Wake up Nexo and load full context (identity, memory, preferences)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "recall",
			Description: "Recall associative memory context for a query",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Query to recall from memory",
					},
					"brief": map[string]interface{}{
						"type":        "boolean",
						"description": "Return brief format (default: false)",
						"default":     false,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "store",
			Description: "Store text in memory as an episode",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to store in memory",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Title for the episode",
					},
					"importance": map[string]interface{}{
						"type":        "number",
						"description": "Importance level (0.0-1.0)",
						"default":     0.5,
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "reflect",
			Description: "Create a reflection linked to current context",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Reflection text/insight",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "journal",
			Description: "Save session journal and consolidate PWS preferences",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "prefs",
			Description: "Show user preferences (PWS)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "stats",
			Description: "Show memory graph statistics",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func handleToolCall(params ToolCallParams) ToolResult {
	var args map[string]interface{}
	if len(params.Arguments) > 0 {
		json.Unmarshal(params.Arguments, &args)
	}

	switch params.Name {
	case "handoff":
		output, err := runNexo("handoff")
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "recall":
		query, _ := args["query"].(string)
		brief, _ := args["brief"].(bool)
		cmd := "recall"
		if brief {
			cmd = "recall-brief"
		}
		output, err := runNexo(cmd, query)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "store":
		text, _ := args["text"].(string)
		title, _ := args["title"].(string)
		importance, _ := args["importance"].(float64)

		cmdArgs := []string{text}
		if title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if importance > 0 {
			cmdArgs = append(cmdArgs, "--importance", fmt.Sprintf("%.2f", importance))
		}

		output, err := runNexo("store", cmdArgs...)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "reflect":
		text, _ := args["text"].(string)
		output, err := runNexo("reflect", text)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "journal":
		output, err := runNexo("journal")
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "prefs":
		output, err := runNexo("prefs")
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	case "stats":
		output, err := runNexo("stats")
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}

	default:
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Unknown tool: " + params.Name}}, IsError: true}
	}
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id interface{}, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -32603,
			"message": message,
		},
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			sendResponse(req.ID, InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				ServerInfo: ServerInfo{
					Name:    ServerName,
					Version: ServerVersion,
				},
			})

		case "notifications/initialized":
			// No response needed for notifications

		case "tools/list":
			sendResponse(req.ID, map[string]interface{}{
				"tools": getTools(),
			})

		case "tools/call":
			var params ToolCallParams
			if paramsData, ok := req.Params.(map[string]interface{}); ok {
				params.Name, _ = paramsData["name"].(string)
				if args, ok := paramsData["arguments"].(map[string]interface{}); ok {
					params.Arguments, _ = json.Marshal(args)
				}
			}
			result := handleToolCall(params)
			sendResponse(req.ID, result)

		default:
			sendError(req.ID, "Method not found: "+req.Method)
		}
	}
}
