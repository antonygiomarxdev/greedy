package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const protocolVersion = "2024-11-05"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Capabilities    serverCapabilities `json:"capabilities"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	Tools     *toolsCap     `json:"tools,omitempty"`
	Resources *resourcesCap `json:"resources,omitempty"`
	Prompts   *promptsCap   `json:"prompts,omitempty"`
}

type toolsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type resourcesCap struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type promptsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type toolsListResult struct {
	Tools []ToolDef `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type resourcesListResult struct {
	Resources []ResourceDef `json:"resources"`
}

type ResourceDef struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type promptsListResult struct {
	Prompts []PromptDef `json:"prompts"`
}

type promptsGetResult struct {
	Messages []promptMessage `json:"messages"`
}

type promptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type resourcesReadResult struct {
	Contents []resourceContent `json:"contents"`
}

type resourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type PromptDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Arguments   []PromptArg `json:"arguments,omitempty"`
}

type PromptArg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *Server) ServeStdio(ctx context.Context) error {
	s.logger.Info("mcp server starting on stdio")

	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	initialized := false

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stdin: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.logger.Warn("invalid json-rpc message", "error", err)
			continue
		}

		if req.ID == nil {
			// Notification — no response needed
			if !initialized && req.Method == "notifications/initialized" {
				initialized = true
			}
			continue
		}

		resp, err := s.handleRequest(ctx, &req, initialized)
		if err != nil {
			resp = &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonRPCError{Code: -32603, Message: err.Error()},
			}
		}

		data, _ := json.Marshal(resp)
		fmt.Fprintf(writer, "%s\n", data)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *jsonRPCRequest, initialized bool) (*jsonRPCResponse, error) {
	if !initialized && req.Method != "initialize" {
		return nil, fmt.Errorf("server not initialized")
	}

	handler, ok := s.rpcHandlers[req.Method]
	if !ok {
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
	return handler(ctx, req)
}

func (s *Server) handleInitialize(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: initResult{
			ProtocolVersion: protocolVersion,
			ServerInfo: serverInfo{
				Name:    "greedy-trader",
				Version: "0.1.0",
			},
			Capabilities: serverCapabilities{
				Tools:     &toolsCap{},
				Resources: &resourcesCap{Subscribe: true},
				Prompts:   &promptsCap{},
			},
		},
	}, nil
}

func (s *Server) handleToolsList(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	tools := s.ListTools()
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  toolsListResult{Tools: tools},
	}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	result, err := s.CallTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolResult{
				Content: []toolContent{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}, nil
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolResult{
			Content: []toolContent{{Type: "text", Text: result}},
		},
	}, nil
}

func (s *Server) handleResourcesRead(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid resource read params: %w", err)
	}

	result, err := s.ReadResource(params.URI)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resourcesReadResult{Contents: []resourceContent{}},
		}, nil
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resourcesReadResult{Contents: []resourceContent{{URI: params.URI, MimeType: "application/json", Text: result}}},
	}, nil
}

func (s *Server) handlePromptsGet(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid prompt get params: %w", err)
	}

	messages, err := s.GetPrompt(params.Name, params.Arguments)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  promptsGetResult{Messages: nil},
		}, nil
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  promptsGetResult{Messages: messages},
	}, nil
}

func (s *Server) handlePing(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{},
	}, nil
}

func (s *Server) handleResourcesList(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	resources := s.ListResources()
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resourcesListResult{Resources: resources},
	}, nil
}

func (s *Server) handlePromptsList(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error) {
	prompts := s.ListPrompts()
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  promptsListResult{Prompts: prompts},
	}, nil
}
