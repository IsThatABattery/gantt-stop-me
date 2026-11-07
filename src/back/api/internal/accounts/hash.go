package accounts

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters recommended by golang.org/x/crypto/argon2: t=1, 64 MiB.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
)

var b64 = base64.RawStdEncoding

// hashSlots caps concurrent argon2 runs, each holding argonMemory KiB, so a burst of logins cannot exhaust RAM.
var hashSlots = make(chan struct{}, runtime.NumCPU())

func idKey(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	return argon2.IDKey(password, salt, time, memory, threads, keyLen)
}

// hashPassword returns an argon2id hash in PHC string format.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := idKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

// checkPassword reports whether password matches a hash from hashPassword.
func checkPassword(hash, password string) bool {
	parts := strings.Split(hash, "$") // "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false
	}
	got := idKey([]byte(password), salt, time, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(got, key) == 1
}

// dummyHash is checked when a login names an unknown email, so it takes as long as a real one.
var dummyHash = sync.OnceValue(func() string { return hashPassword(rand.Text()) })
