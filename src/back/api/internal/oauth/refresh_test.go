package oauth_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"gantt-stop-me/api/internal/oauth"
)

func TestDue(t *testing.T) {
	at := func(d time.Duration) *time.Time { e := time.Now().Add(d); return &e }
	for _, c := range []struct {
		name    string
		expires *time.Time
		want    bool
	}{
		{"never expires", nil, false},
		{"8 hours away", at(8 * time.Hour), false},
		{"6 minutes away", at(6 * time.Minute), false},
		{"4 minutes away", at(4 * time.Minute), true},
		{"expired", at(-time.Minute), true},
	} {
		if got := oauth.Due(c.expires); got != c.want {
			t.Errorf("%s: Due = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var gates oauth.Gates[int64]
		ctx := t.Context()
		unlock, err := gates.Wait(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}

		// Another caller for key 1 waits until the first unlocks; one for key 2 goes through.
		through := make(chan func(), 1)
		go func() {
			u, err := gates.Wait(ctx, 1)
			if err != nil {
				t.Error(err)
			}
			through <- u
		}()
		other, err := gates.Wait(ctx, 2)
		if err != nil {
			t.Fatalf("Wait for another key: %v", err)
		}
		other()
		synctest.Wait()
		if len(through) != 0 {
			t.Fatal("a second caller went through while the first held the gate")
		}

		// A caller that gives up while it waits goes nowhere.
		cctx, cancel := context.WithCancel(ctx)
		gaveUp := make(chan error, 1)
		go func() {
			u, err := gates.Wait(cctx, 1)
			if u != nil {
				t.Error("Wait for a caller that gave up returned unlock")
			}
			gaveUp <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-gaveUp; !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait for a caller that gave up = %v, want context.Canceled", err)
		}

		// Unlocking lets the next caller through, and it holds the gate in turn.
		unlock()
		second := <-through
		go func() {
			u, err := gates.Wait(ctx, 1)
			if err != nil {
				t.Error(err)
			}
			through <- u
		}()
		synctest.Wait()
		if len(through) != 0 {
			t.Fatal("a third caller went through while the second held the gate")
		}
		second()
		(<-through)()
	})
}

func TestBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var b oauth.Backoff[int64]
		if err := b.Failed(1); err != nil {
			t.Fatalf("Failed before any failure = %v, want nil", err)
		}
		down := errors.New("token endpoint down")
		b.Fail(1, down)
		if err := b.Failed(1); err != down {
			t.Fatalf("Failed = %v, want %v", err, down)
		}
		if err := b.Failed(2); err != nil {
			t.Fatalf("Failed for another key = %v, want nil", err)
		}

		time.Sleep(oauth.RefreshBackoff - time.Second)
		if err := b.Failed(1); err != down {
			t.Fatalf("Failed a second before the backoff ends = %v, want %v", err, down)
		}
		time.Sleep(time.Second)
		if err := b.Failed(1); err != nil {
			t.Fatalf("Failed once the backoff ends = %v, want nil", err)
		}

		// A new failure starts a new backoff, which Clear ends.
		b.Fail(1, down)
		b.Fail(2, down)
		b.Clear(1)
		if err := b.Failed(1); err != nil {
			t.Fatalf("Failed after Clear = %v, want nil", err)
		}
		if err := b.Failed(2); err != down {
			t.Fatalf("Failed for another key after Clear = %v, want %v", err, down)
		}
	})
}
