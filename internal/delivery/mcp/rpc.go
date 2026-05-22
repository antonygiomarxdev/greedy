package mcp

import "context"

type rpcHandlerFunc func(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error)

func (s *Server) registerRPCHandlers() {
	s.rpcHandlers = map[string]rpcHandlerFunc{
		"initialize":     s.handleInitialize,
		"tools/list":     s.handleToolsList,
		"tools/call":     s.handleToolsCall,
		"resources/list": s.handleResourcesList,
		"prompts/list":   s.handlePromptsList,
	}
}
