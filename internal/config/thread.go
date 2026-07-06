package config

import (
	"crypto/rand"
	"encoding/hex"
)

// NewThreadID generates a random thread ID for tracking requests
func NewThreadID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
