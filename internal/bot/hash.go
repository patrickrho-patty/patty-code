package bot

import (
	"crypto/sha256"
	"encoding/hex"
)

// [hashID / ID ，。]
func hashID(id string) string {
	if id == "" {
		return ""
	}
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:])[:12]
}
