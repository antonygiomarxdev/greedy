package mcp

import "context"

type rpcHandlerFunc func(ctx context.Context, req *jsonRPCRequest) (*jsonRPCResponse, error)
type rpcFactory func(s *Server) rpcHandlerFunc

var rpcFactories = map[string]rpcFactory{
	"initialize":     func(s *Server) rpcHandlerFunc { return s.handleInitialize },
	"tools/list":     func(s *Server) rpcHandlerFunc { return s.handleToolsList },
	"tools/call":     func(s *Server) rpcHandlerFunc { return s.handleToolsCall },
	"resources/list": func(s *Server) rpcHandlerFunc { return s.handleResourcesList },
	"prompts/list":   func(s *Server) rpcHandlerFunc { return s.handlePromptsList },
}

func (s *Server) registerRPCHandlers() {
	s.rpcHandlers = make(map[string]rpcHandlerFunc, len(rpcFactories))
	for method, factory := range rpcFactories {
		s.rpcHandlers[method] = factory(s)
	}
}
