package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// DeviceAuth returns an Echo middleware that validates the provisioning token.
func DeviceAuth(provisioningToken string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing provisioning token")
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token != provisioningToken {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid provisioning token")
			}

			return next(c)
		}
	}
}
