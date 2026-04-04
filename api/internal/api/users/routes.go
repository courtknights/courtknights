package users

import "github.com/labstack/echo/v4"

// Routes implements api.RouteRegistrar for the users feature.
type Routes struct {
	handler *Handler
}

// NewRoutes returns a Routes for the given Handler.
func NewRoutes(h *Handler) *Routes {
	return &Routes{handler: h}
}

// Register mounts all user management routes on the given group.
// Called by the router for the protected /api/v1 prefix (JWT middleware applied).
func (r *Routes) Register(g *echo.Group) {
	g.GET("/users", r.handler.listUsers)
	g.GET("/users/me", r.handler.me)
	g.PUT("/users/:id/role", r.handler.updateRole)

	// Profile endpoints — all require JWT (applied at group level).
	g.GET("/users/me/profile", r.handler.getMyProfile)
	g.PUT("/users/me/profile", r.handler.updateMyProfile)
	g.GET("/users/:id/profile", r.handler.getUserProfile)
}
