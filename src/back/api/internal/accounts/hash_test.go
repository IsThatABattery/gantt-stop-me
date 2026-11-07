package accounts

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
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

// Every login and signup runs one hash of argonMemory, so a burst of them must not hold one each at once.
func TestHashBurstMemoryIsBounded(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(10)) // keep freed blocks from inflating HeapInuse
	h := dummyHash()
	runtime.GC()
	var peak uint64
	done, sampled := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(sampled)
		var ms runtime.MemStats
		for {
			select {
			case <-done:
				return
			default:
			}
			runtime.ReadMemStats(&ms)
			peak = max(peak, ms.HeapInuse)
			time.Sleep(2 * time.Millisecond)
		}
	}()
	n := 4 * runtime.NumCPU()
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { checkPassword(h, "x") })
	}
	wg.Wait()
	close(done)
	<-sampled
	if limit := uint64(2*runtime.NumCPU()) * argonMemory * 1024; peak > limit {
		t.Fatalf("%d concurrent hashes peaked at %d MiB, want <= %d MiB", n, peak>>20, limit>>20)
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
