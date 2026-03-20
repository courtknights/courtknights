package auth

import "github.com/labstack/echo/v4"

// Routes implements api.RouteRegistrar for the auth feature.
type Routes struct {
	handler *Handler
}

// NewRoutes returns a Routes for the given Handler.
func NewRoutes(h *Handler) *Routes {
	return &Routes{handler: h}
}

// Register mounts all auth routes on the given group.
// Called by the router for the public /auth prefix (no JWT middleware).
func (r *Routes) Register(g *echo.Group) {
	g.GET("/:provider", r.handler.redirectToProvider)
	g.GET("/:provider/callback", r.handler.oauthCallback)
	g.POST("/device", r.handler.deviceAuth)
	g.POST("/device/token", r.handler.deviceToken)
}
