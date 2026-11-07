package accounts

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	h := hashPassword("correct horse")
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("hash %q is not argon2id PHC", h)
	}
	if !checkPassword(h, "correct horse") {
		t.Fatal("right password rejected")
	}
	if checkPassword(h, "correct horsf") {
		t.Fatal("wrong password accepted")
	}
	if h == hashPassword("correct horse") {
		t.Fatal("two hashes of one password are equal; salt is not random")
	}
}

func TestCheckPasswordRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=65536,t=1,p=4$c2FsdA$a2V5",
		"$argon2id$v=18$m=65536,t=1,p=4$c2FsdA$a2V5",
		"$argon2id$v=19$m=x,t=1,p=4$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=1,p=4$!!$a2V5",
		"$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$",
	} {
		if checkPassword(h, "anything") {
			t.Errorf("checkPassword accepted malformed hash %q", h)
		}
	}
}
