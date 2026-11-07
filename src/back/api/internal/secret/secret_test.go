package secret

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func newBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSealOpenRoundTrip(t *testing.T) {
	b := newBox(t)
	sealed := b.Seal([]byte("ghp_token"))
	if bytes.Contains(sealed, []byte("ghp_token")) {
		t.Fatal("sealed value contains the plaintext")
	}
	got, err := b.Open(sealed)
	if err != nil || string(got) != "ghp_token" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	if bytes.Equal(sealed, b.Seal([]byte("ghp_token"))) {
		t.Fatal("two seals of one value are equal; nonce is not random")
	}
}

func TestOpenRejectsTamperingAndOtherKeys(t *testing.T) {
	b := newBox(t)
	sealed := b.Seal([]byte("ghp_token"))
	sealed[len(sealed)-1] ^= 1
	if _, err := b.Open(sealed); err == nil {
		t.Fatal("Open accepted a tampered value")
	}
	if _, err := newBox(t).Open(b.Seal([]byte("x"))); err == nil {
		t.Fatal("Open accepted a value sealed with another key")
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := New(make([]byte, 31)); err == nil {
		t.Fatal("New accepted a 31-byte key")
	}
}
