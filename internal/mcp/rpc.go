package mcp

import "context"

type rpcHandlerFunc func(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error)
type rpcFactory func(s *Server) rpcHandlerFunc

var rpcFactories = map[string]rpcFactory{
	"initialize":     func(s *Server) rpcHandlerFunc { return s.handleInitialize },
	"ping":           func(s *Server) rpcHandlerFunc { return s.handlePing },
	"tools/list":     func(s *Server) rpcHandlerFunc { return s.handleToolsList },
	"tools/call":     func(s *Server) rpcHandlerFunc { return s.handleToolsCall },
	"resources/list": func(s *Server) rpcHandlerFunc { return s.handleResourcesList },
	"resources/read": func(s *Server) rpcHandlerFunc { return s.handleResourcesRead },
	"prompts/list":   func(s *Server) rpcHandlerFunc { return s.handlePromptsList },
	"prompts/get":    func(s *Server) rpcHandlerFunc { return s.handlePromptsGet },
}

func (s *Server) registerRPCHandlers() {
	s.rpcHandlers = make(map[string]rpcHandlerFunc, len(rpcFactories))
	for method, factory := range rpcFactories {
		s.rpcHandlers[method] = factory(s)
	}
}
