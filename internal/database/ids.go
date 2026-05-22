package database

import (
	"crypto/rand"
	"encoding/hex"
)

func newPrefixedID(prefix string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}
