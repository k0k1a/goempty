// Package cursor encodes and authenticates pagination cursors.
package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cast"
)

// ErrInvalidCursor indicates an invalid encoding or signature.
var ErrInvalidCursor = errors.New("invalid cursor")

// Codec signs a position for a specific list scope and user.
// Callers must construct it with a non-empty, unpredictable key and a scope
// that distinguishes the list, environment and format version.
type Codec[T any] struct {
	key   []byte
	scope string
}

// NewCodec constructs a codec that serializes positions as JSON.
func NewCodec[T any](key, scope string) Codec[T] {
	return Codec[T]{key: []byte(key), scope: scope}
}

// Encode escapes position into a signed, URL-safe token bound to userID.
// Pass 0 for lists that are not scoped to a single user.
func (c Codec[T]) Encode(position T, userID int64) (string, error) {
	payload, err := json.Marshal(position)
	if err != nil {
		return "", err
	}
	signed := append(payload, c.signature(payload, userID)...)
	return base64.RawURLEncoding.EncodeToString(signed), nil
}

// Decode verifies the token and returns the position it carries.
// Any encoding, signature, scope or user mismatch returns ErrInvalidCursor.
func (c Codec[T]) Decode(token string, userID int64) (T, error) {
	var zero T
	encoding := base64.RawURLEncoding.Strict()
	signed, err := encoding.DecodeString(token)
	if err != nil || len(signed) < sha256.Size || len(token) != encoding.EncodedLen(len(signed)) {
		return zero, ErrInvalidCursor
	}
	signatureOffset := len(signed) - sha256.Size
	payload, signature := signed[:signatureOffset], signed[signatureOffset:]
	if !hmac.Equal(signature, c.signature(payload, userID)) {
		return zero, ErrInvalidCursor
	}
	var position T
	if err := json.Unmarshal(payload, &position); err != nil {
		return zero, ErrInvalidCursor
	}
	return position, nil
}

func (c Codec[T]) signature(payload []byte, userID int64) []byte {
	mac := hmac.New(sha256.New, c.key)
	fmt.Fprintf(mac, "%s:%q:", c.scope, cast.ToString(userID))
	mac.Write(payload)
	return mac.Sum(nil)
}
