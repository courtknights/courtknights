// Package api wires together the Echo instance, middleware, and route groups.
package api

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// RouteRegistrar is implemented by each feature's routes file.
// It receives the group it should register its routes on.
type RouteRegistrar interface {
	Register(g *echo.Group)
}

// Router holds the Echo instance and exposes mount helpers.
type Router struct {
	echo *echo.Echo
}

// New creates a pre-configured Echo instance with logging and recovery middleware.
func New() *Router {
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	return &Router{echo: e}
}

// Echo returns the underlying Echo instance (e.g. for starting the server).
func (r *Router) Echo() *echo.Echo {
	return r.echo
}

// MountPublic registers routes on a group without JWT middleware.
// Used for /auth/* endpoints that must be reachable without a token.
func (r *Router) MountPublic(prefix string, registrar RouteRegistrar) {
	g := r.echo.Group(prefix)
	registrar.Register(g)
}

// MountProtected registers routes on /api/v1 with JWT middleware applied.
func (r *Router) MountProtected(prefix string, jwtMW echo.MiddlewareFunc, registrar RouteRegistrar) {
	g := r.echo.Group(prefix, jwtMW)
	registrar.Register(g)
}
