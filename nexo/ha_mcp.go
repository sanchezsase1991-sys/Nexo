package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	HAServerName    = "home-assistant"
	HAServerVersion = "1.0.0"
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

var (
	haURL   string
	haToken string
	client  = &http.Client{Timeout: 10 * time.Second}
)

func init() {
	haURL = os.Getenv("HA_URL")
	if haURL == "" {
		haURL = "http://localhost:8123"
	}
	haToken = os.Getenv("HA_TOKEN")
}

func haRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, haURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+haToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func getHATools() []Tool {
	return []Tool{
		{
			Name:        "ha_get_states",
			Description: "Get all entity states from Home Assistant",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional filter (e.g., 'light.', 'sensor.')",
					},
				},
			},
		},
		{
			Name:        "ha_get_state",
			Description: "Get state of a specific entity",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Entity ID (e.g., 'light.living_room')",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_call_service",
			Description: "Call a Home Assistant service",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"domain": map[string]interface{}{
						"type":        "string",
						"description": "Service domain (e.g., 'light', 'switch')",
					},
					"service": map[string]interface{}{
						"type":        "string",
						"description": "Service name (e.g., 'turn_on', 'turn_off')",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Target entity ID",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "Optional service data",
					},
				},
				"required": []string{"domain", "service", "entity_id"},
			},
		},
		{
			Name:        "ha_light_on",
			Description: "Turn on a light",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Light entity ID",
					},
					"brightness": map[string]interface{}{
						"type":        "number",
						"description": "Brightness (0-255)",
					},
					"color_temp": map[string]interface{}{
						"type":        "number",
						"description": "Color temperature in mireds",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_light_off",
			Description: "Turn off a light",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Light entity ID",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_switch_on",
			Description: "Turn on a switch",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Switch entity ID",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_switch_off",
			Description: "Turn off a switch",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Switch entity ID",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_camera_snapshot",
			Description: "Get a camera snapshot",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Camera entity ID",
					},
				},
				"required": []string{"entity_id"},
			},
		},
		{
			Name:        "ha_get_areas",
			Description: "Get all areas/rooms",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "ha_get_devices",
			Description: "Get all devices",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "ha_get_automations",
			Description: "Get all automations",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "ha_trigger_automation",
			Description: "Trigger an automation",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Automation entity ID",
					},
				},
				"required": []string{"entity_id"},
			},
		},
	}
}

func handleHACall(name string, args map[string]interface{}) ToolResult {
	switch name {
	case "ha_get_states":
		filter, _ := args["filter"].(string)
		data, err := haRequest("GET", "/api/states", nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		var states []map[string]interface{}
		json.Unmarshal(data, &states)

		if filter != "" {
			var filtered []map[string]interface{}
			for _, s := range states {
			 eid, _ := s["entity_id"].(string)
				if strings.HasPrefix(eid, filter) {
					filtered = append(filtered, s)
				}
			}
			states = filtered
		}

		result, _ := json.MarshalIndent(states, "", "  ")
		return ToolResult{Content: []TextContent{{Type: "text", Text: string(result)}}}

	case "ha_get_state":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("GET", "/api/states/"+entityID, nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: string(data)}}}

	case "ha_call_service":
		domain, _ := args["domain"].(string)
		service, _ := args["service"].(string)
		entityID, _ := args["entity_id"].(string)
		serviceData, _ := args["data"].(map[string]interface{})

		payload := map[string]interface{}{
			"entity_id": entityID,
		}
		if serviceData != nil {
			payload["data"] = serviceData
		}

		data, err := haRequest("POST", "/api/services/"+domain+"/"+service, payload)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "OK: " + string(data)}}}

	case "ha_light_on":
		entityID, _ := args["entity_id"].(string)
		payload := map[string]interface{}{}
		if brightness, ok := args["brightness"].(float64); ok {
			payload["brightness"] = int(brightness)
		}
		if colorTemp, ok := args["color_temp"].(float64); ok {
			payload["color_temp"] = int(colorTemp)
		}
		data, err := haRequest("POST", "/api/services/light/turn_on", map[string]interface{}{
			"entity_id": entityID,
			"data":      payload,
		})
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Light ON: " + string(data)}}}

	case "ha_light_off":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("POST", "/api/services/light/turn_off", map[string]interface{}{
			"entity_id": entityID,
		})
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Light OFF: " + string(data)}}}

	case "ha_switch_on":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("POST", "/api/services/switch/turn_on", map[string]interface{}{
			"entity_id": entityID,
		})
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Switch ON: " + string(data)}}}

	case "ha_switch_off":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("POST", "/api/services/switch/turn_off", map[string]interface{}{
			"entity_id": entityID,
		})
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Switch OFF: " + string(data)}}}

	case "ha_camera_snapshot":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("GET", "/api/camera_proxy/"+entityID, nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Snapshot captured (%d bytes)", len(data))}}}

	case "ha_get_areas":
		data, err := haRequest("GET", "/api/config/area_registry", nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: string(data)}}}

	case "ha_get_devices":
		data, err := haRequest("GET", "/api/config/device_registry", nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: string(data)}}}

	case "ha_get_automations":
		data, err := haRequest("GET", "/api/config/automation/config", nil)
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: string(data)}}}

	case "ha_trigger_automation":
		entityID, _ := args["entity_id"].(string)
		data, err := haRequest("POST", "/api/services/automation/trigger", map[string]interface{}{
			"entity_id": entityID,
		})
		if err != nil {
			return ToolResult{Content: []TextContent{{Type: "text", Text: "Error: " + err.Error()}}, IsError: true}
		}
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Triggered: " + string(data)}}}

	default:
		return ToolResult{Content: []TextContent{{Type: "text", Text: "Unknown tool: " + name}}, IsError: true}
	}
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

func RunHAMCP() {
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
			sendMCPResponse(req.ID, InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				ServerInfo: ServerInfo{
					Name:    HAServerName,
					Version: HAServerVersion,
				},
			})

		case "notifications/initialized":
			// No response needed

		case "tools/list":
			sendMCPResponse(req.ID, map[string]interface{}{
				"tools": getHATools(),
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
			result := handleHACall(params.Name, params.Arguments)
			sendMCPResponse(req.ID, result)

		default:
			sendMCPResponse(req.ID, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found: " + req.Method,
				},
			})
		}
	}
}

func main() {
	if haToken == "" {
		fmt.Fprintln(os.Stderr, "Error: HA_TOKEN not set")
		fmt.Fprintln(os.Stderr, "Usage: HA_TOKEN=<token> HA_URL=<url> ./ha-mcp")
		os.Exit(1)
	}
	RunHAMCP()
}
