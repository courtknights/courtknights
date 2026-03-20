// Package common provides shared HTTP middleware for the CourtKnights API.
package common

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/courtknights/courtknights/internal/application/auth"
	"github.com/courtknights/courtknights/internal/domain/ckerrors"
)

// ContextKeyRole is the echo.Context key for the authenticated user's role.
const (
	ContextKeySub   = "sub"
	ContextKeyEmail = "email"
	ContextKeyRole  = "role"
)

// JWTMiddleware returns an Echo middleware that validates the Bearer JWT and
// sets sub, email, and role on the context. Returns 401 on any failure.
func JWTMiddleware(jwt auth.JWTManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, ckerrors.ErrUnauthorized.Error())
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := jwt.Validate(tokenStr)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, ckerrors.ErrUnauthorized.Error())
			}

			c.Set(ContextKeySub, claims.Subject)
			c.Set(ContextKeyEmail, claims.Email)
			c.Set(ContextKeyRole, string(claims.Role))

			return next(c)
		}
	}
}
