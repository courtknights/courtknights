package pats

import "github.com/labstack/echo/v4"

// Routes implements api.RouteRegistrar for the pats feature.
type Routes struct {
	handler *Handler
}

// NewRoutes returns a Routes for the given Handler.
func NewRoutes(h *Handler) *Routes {
	return &Routes{handler: h}
}

// Register mounts all PAT management routes on the given group.
// Called by the router for the protected /api/v1 prefix (JWT middleware applied).
func (r *Routes) Register(g *echo.Group) {
	g.POST("/pats", r.handler.createPAT)
	g.GET("/pats", r.handler.listPATs)
	g.DELETE("/pats/:id", r.handler.revokePAT)
}
