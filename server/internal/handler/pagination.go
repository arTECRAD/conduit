package handler

import (
	"encoding/base64"
	"fmt"
)

// encodeCursor encodes a cursor value to an opaque base64 token.
func encodeCursor(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

// decodeCursor decodes an opaque cursor token back to its value.
func decodeCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("invalid cursor: %w", err)
	}
	return string(b), nil
}
