package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	MCPName    = "nexo-memory"
	MCPVersion = "5.0.0"
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

func getMCPTools() []Tool {
	return []Tool{
		{
			Name:        "handoff",
			Description: "🌅 Protocolo de despertar - carga identidad + memoria + preferencias",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "recall",
			Description: "Recuperar contexto asociativo",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Query para recuperar de memoria",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "recall-brief",
			Description: "Recuperar en formato breve (para agente)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Query para recuperar",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "recall-deep",
			Description: "Recall profundo con cadenas multihop",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Query para recall profundo",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "store",
			Description: "Almacenar en memoria",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Texto a almacenar",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Título del episodio",
					},
					"importance": map[string]interface{}{
						"type":        "number",
						"description": "Importancia (0.0-1.0)",
						"default":     0.5,
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "reflect",
			Description: "Crear reflexión asociada al contexto actual",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Insight o reflexión",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "journal",
			Description: "Guardar diario de sesión (resumen + cierre)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "consolidate",
			Description: "Ejecutar consolidación (sueño)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "stats",
			Description: "Mostrar estadísticas de la base de datos",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "prefs",
			Description: "Mostrar preferencias del usuario (PWS)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "align",
			Description: "Ejecutar análisis de alineamiento",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Query para análisis",
					},
				},
			},
		},
		{
			Name:        "pws-session",
			Description: "Consolidar y mostrar resumen de sesión PWS",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "pattern",
			Description: "Registrar patrón de comportamiento",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Texto del patrón",
					},
				},
				"required": []string{"text"},
			},
		},
	}
}

func handleMCPCall(name string, args map[string]interface{}) ToolResult {
	cfg := &DefaultConfig
	db, err := OpenDB(cfg)
	if err != nil {
		return ToolResult{Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Error abriendo BD: %v", err)}}, IsError: true}
	}
	defer db.Close()

	var output string

	switch name {
	case "handoff":
		state, err := Handoff(db, cfg)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
		}
		// Capture output
	 output = captureOutput(func() { PrintHandoff(state) })

	case "recall":
		query, _ := args["query"].(string)
		if query == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "query requerido"}}, IsError: true}
		}
		output = captureOutput(func() { Recall(db, cfg, query) })

	case "recall-brief":
		query, _ := args["query"].(string)
		if query == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "query requerido"}}, IsError: true}
		}
		output = captureOutput(func() { RecallBrief(db, cfg, query) })

	case "recall-deep":
		query, _ := args["query"].(string)
		if query == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "query requerido"}}, IsError: true}
		}
		output = captureOutput(func() { DeepRecall(db, cfg, query) })

	case "store":
		text, _ := args["text"].(string)
		title, _ := args["title"].(string)
		importance, _ := args["importance"].(float64)
		if importance == 0 {
			importance = 0.5
		}
		if text == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "text requerido"}}, IsError: true}
		}
		output = captureOutput(func() { Store(db, text, title, importance) })

	case "reflect":
		text, _ := args["text"].(string)
		if text == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "text requerido"}}, IsError: true}
		}
		output = captureOutput(func() { Reflect(db, text) })

	case "journal":
		output = captureOutput(func() { Journal(db) })

	case "consolidate":
		output = captureOutput(func() { Consolidate(db) })

	case "stats":
		output = captureOutput(func() { Stats(db) })

	case "prefs":
		output = captureOutput(func() { ShowPreferences(db) })

	case "align":
		query, _ := args["query"].(string)
		if query != "" {
			UpdateStyleVector(db, query)
		}
		bias, errAlign := GetAlignmentBias(db, nil)
		if errAlign != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: errAlign.Error()}}, IsError: true}
		}
		output = fmt.Sprintf("Confianza: %.0f%%\nEstilo dominante: %s (%.2f)\nNodos con peso ajustado: %d",
			bias.Confidence*100, bias.StyleBias.Dimension, bias.StyleBias.Value, len(bias.NodeWeights))

	case "pws-session":
		output = captureOutput(func() { ShowSessionSummary(db) })

	case "pattern":
		text, _ := args["text"].(string)
		if text == "" {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "text requerido"}}, IsError: true}
		}
		pattern, errPattern := AnalyzePattern(db, text, PatternTypeQuery)
		if errPattern != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: errPattern.Error()}}, IsError: true}
		}
		output = fmt.Sprintf("Patrón registrado:\n  ID: %s\n  Tipo: %s\n  Observaciones: %d\n  Peso: %.2f",
			pattern.ID, pattern.PatternType, pattern.ObservationCount, pattern.Weight)

	default:
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Herramienta desconocida: " + name}}, IsError: true}
	}

	if err != nil {
		return ToolResult{Content: []TextContent{{Type: "text", Text: err.Error()}}, IsError: true}
	}

	if output == "" {
		output = "OK"
	}

	return ToolResult{Content: []TextContent{{Type: "text", Text: output}}}
}

func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	tmp := make([]byte, 8192)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	r.Close()
	return buf.String()
}

func sendMCPResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func RunMCP() {
	// Log to stderr for debugging
	fmt.Fprintln(os.Stderr, "🧠 Nexo MCP Server starting (EOF-loop mode)...")

	// Use raw file descriptor read instead of bufio.Scanner
	// This allows us to handle EOF by recreating the reader
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read line by line
		line, err := reader.ReadString('\n')
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Fprintln(os.Stderr, "🔄 EOF detected - resetting reader (waiting for next connection)...")
				// Create a new reader on the same fd - this works because
				// OpenCode may reconnect stdin
				reader = bufio.NewReader(os.Stdin)
				// Brief pause to avoid tight loop
				// In practice, the process will be killed/restarted by OpenCode
				// but if it survives, we keep trying
				continue
			}
			fmt.Fprintf(os.Stderr, "❌ Read error: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fmt.Fprintf(os.Stderr, "📥 Request: %s\n", truncate(line, 100))

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Parse error: %v\n", err)
			continue
		}

		switch req.Method {
		case "initialize":
			fmt.Fprintln(os.Stderr, "📋 Method: initialize")
			sendMCPResponse(req.ID, InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				ServerInfo: ServerInfo{
					Name:    MCPName,
					Version: MCPVersion,
				},
			})

		case "notifications/initialized":
			fmt.Fprintln(os.Stderr, "📋 Method: notifications/initialized")
			// No response needed

		case "tools/list":
			fmt.Fprintln(os.Stderr, "📋 Method: tools/list")
			sendMCPResponse(req.ID, map[string]interface{}{
				"tools": getMCPTools(),
			})

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
			fmt.Fprintf(os.Stderr, "📋 Method: tools/call → %s\n", params.Name)
			result := handleMCPCall(params.Name, params.Arguments)
			fmt.Fprintf(os.Stderr, "📤 Response sent for %s\n", params.Name)
			sendMCPResponse(req.ID, result)

		default:
			fmt.Fprintf(os.Stderr, "📋 Method: %s (unknown)\n", req.Method)
			sendMCPResponse(req.ID, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found: " + req.Method,
				},
			})
		}
	}

	fmt.Fprintln(os.Stderr, "🧠 Nexo MCP Server stopped.")
}
