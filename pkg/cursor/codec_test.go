package cursor

import (
	"encoding/base64"
	"errors"
	"testing"
)

type position struct {
	TimeMillis int64
	ID         int64
}

func TestCodec(t *testing.T) {
	codec := NewCodec[position]("test-key", "posts:v1:dev")
	want := position{TimeMillis: 1700000000000, ID: 42}

	token, err := codec.Encode(want, 7)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	got, err := codec.Decode(token, 7)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got != want {
		t.Fatalf("Decode() = %+v, want %+v", got, want)
	}

	zeroUserToken, err := codec.Encode(want, 0)
	if err != nil {
		t.Fatalf("Encode() without user error = %v", err)
	}
	if _, err := codec.Decode(zeroUserToken, 0); err != nil {
		t.Fatalf("Decode() without user error = %v", err)
	}
}

func TestCodecRejectsInvalidTokens(t *testing.T) {
	codec := NewCodec[position]("test-key", "posts:v1:dev")
	otherScope := NewCodec[position]("test-key", "comments:v1:dev")
	token, err := codec.Encode(position{ID: 1}, 7)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	signed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode raw token: %v", err)
	}

	invalidSignature := append([]byte(nil), signed...)
	invalidSignature[len(invalidSignature)-1] ^= 0xff
	padded := base64.URLEncoding.EncodeToString(signed)

	for _, tc := range []struct {
		name   string
		codec  Codec[position]
		token  string
		userID int64
	}{
		{"other user", codec, token, 8},
		{"other scope", otherScope, token, 7},
		{"tampered signature", codec, base64.RawURLEncoding.EncodeToString(invalidSignature), 7},
		{"padded encoding", codec, padded, 7},
		{"empty token", codec, "", 7},
		{"malformed base64", codec, "not*base64", 7},
		{"shorter than signature", codec, base64.RawURLEncoding.EncodeToString([]byte("short")), 7},
		{"not JSON payload", codec, signRaw(codec, []byte("plain"), 7), 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.codec.Decode(tc.token, tc.userID); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("Decode() error = %v, want ErrInvalidCursor", err)
			}
		})
	}
}

func signRaw(codec Codec[position], payload []byte, userID int64) string {
	return base64.RawURLEncoding.EncodeToString(append(payload, codec.signature(payload, userID)...))
}
