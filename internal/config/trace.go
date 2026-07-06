package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateTraceparent generates a W3C traceparent header value
func GenerateTraceparent() string {
	version := "00"
	traceID := randomHex(16)
	parentID := randomHex(8)
	traceFlags := "01"
	
	return fmt.Sprintf("%s-%s-%s-%s", version, traceID, parentID, traceFlags)
}

// randomHex generates random hex string of specified length
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
