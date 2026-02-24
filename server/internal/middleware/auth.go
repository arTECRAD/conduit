package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// ContextKeyUserID is the context key for the authenticated user's UUID string.
const ContextKeyUserID = "userID"

// TokenValidator validates a JWT and returns the subject (user ID as string).
type TokenValidator interface {
	ValidateAccessToken(token string) (interface{ String() string }, error)
}

// JWTAuthFunc is a function that validates a JWT and returns the user ID string.
type JWTAuthFunc func(token string) (string, error)

// JWTAuth returns an Echo middleware that validates JWT Bearer tokens.
func JWTAuth(validateFn JWTAuthFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing or invalid Authorization header")
			}

			token := strings.TrimPrefix(header, "Bearer ")
			userID, err := validateFn(token)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			c.Set(ContextKeyUserID, userID)
			return next(c)
		}
	}
}
