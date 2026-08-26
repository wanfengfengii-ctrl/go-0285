package application

import (
	"crypto/sha256"
	"encoding/hex"
)

// digestOf returns a short content digest for a string used in evidence
// records and lease keys.
func digestOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
