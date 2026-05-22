package mcp

func (s *Server) ListResources() []ResourceDef {
	return []ResourceDef{
		{
			URI:         "portfolio://summary",
			Name:        "Portfolio Summary",
			Description: "Full portfolio snapshot with total P&L, exposure, and positions",
			MimeType:    "application/json",
		},
		{
			URI:         "market://prices/{symbol}",
			Name:        "Live Price",
			Description: "Real-time price for any trading symbol",
			MimeType:    "application/json",
		},
		{
			URI:         "bot://{id}/status",
			Name:        "Bot Status",
			Description: "Detailed status, open orders, and P&L for a specific bot",
			MimeType:    "application/json",
		},
	}
}
