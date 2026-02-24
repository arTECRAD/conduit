package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogger returns an Echo middleware that logs each request using slog.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			req := c.Request()
			res := c.Response()

			statusCode := res.Status
			if he, ok := err.(*echo.HTTPError); ok {
				statusCode = he.Code
			}

			level := slog.LevelInfo
			if statusCode >= 500 {
				level = slog.LevelError
			} else if statusCode >= 400 {
				level = slog.LevelWarn
			}

			slog.LogAttrs(req.Context(), level, "http request",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", statusCode),
				slog.Duration("duration", duration),
				slog.String("ip", c.RealIP()),
				slog.String("request_id", res.Header().Get(echo.HeaderXRequestID)),
			)

			return err
		}
	}
}
