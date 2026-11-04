package cardinalitylimitfilter

import (
	"bytes"
	"sync"
	"testing"
)

func TestNewRejectsNonPositiveLimit(t *testing.T) {
	for _, limit := range []int{0, -1, -100} {
		if _, err := New(limit, Drop, nil); err == nil {
			t.Fatalf("New(%d) = nil error, want error", limit)
		}
	}
}

func TestAllowAdmitsUpToLimit(t *testing.T) {
	f, err := New(3, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i, labels := range [][]byte{[]byte("a"), []byte("b"), []byte("c")} {
		got, ok := f.Allow(labels)
		if !ok {
			t.Fatalf("Allow(#%d) = false, want true", i)
		}
		if !bytes.Equal(got, labels) {
			t.Fatalf("Allow(#%d) = %q, want %q", i, got, labels)
		}
	}

	if got := f.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
}

func TestAllowDropsBeyondLimit(t *testing.T) {
	f, err := New(2, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := f.Allow([]byte("a")); !ok {
		t.Fatal("first Allow = false, want true")
	}
	if _, ok := f.Allow([]byte("b")); !ok {
		t.Fatal("second Allow = false, want true")
	}

	got, ok := f.Allow([]byte("c"))
	if ok {
		t.Fatal("third Allow = true, want false")
	}
	if got != nil {
		t.Fatalf("third Allow returned %q, want nil", got)
	}

	// An already-seen label set is still admitted even when the filter is full.
	if _, ok := f.Allow([]byte("a")); !ok {
		t.Fatal("re-Allow of seen label = false, want true")
	}
}

func TestAllowAggregatesBeyondLimit(t *testing.T) {
	f, err := New(2, Aggregate, []byte("other"))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := f.Allow([]byte("a")); !ok {
		t.Fatal("first Allow = false, want true")
	}
	if _, ok := f.Allow([]byte("b")); !ok {
		t.Fatal("second Allow = false, want true")
	}

	got, ok := f.Allow([]byte("c"))
	if !ok {
		t.Fatal("third Allow = false, want true")
	}
	if !bytes.Equal(got, []byte("other")) {
		t.Fatalf("third Allow = %q, want %q", got, "other")
	}

	// The overflow label set is not itself recorded, so a later distinct label
	// still maps to the overflow set rather than being admitted.
	got, ok = f.Allow([]byte("d"))
	if !ok {
		t.Fatal("fourth Allow = false, want true")
	}
	if !bytes.Equal(got, []byte("other")) {
		t.Fatalf("fourth Allow = %q, want %q", got, "other")
	}
}

func TestAllowReturnsCopy(t *testing.T) {
	f, err := New(1, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}

	in := []byte("a")
	out, ok := f.Allow(in)
	if !ok {
		t.Fatal("Allow = false, want true")
	}
	out[0] = 'z'
	if !bytes.Equal(in, []byte("a")) {
		t.Fatalf("input mutated to %q, want %q", in, "a")
	}

	again, ok := f.Allow(in)
	if !ok {
		t.Fatal("second Allow = false, want true")
	}
	if !bytes.Equal(again, []byte("a")) {
		t.Fatalf("second Allow = %q, want %q", again, "a")
	}
}

func TestAllowDistinguishesEmptyAndNil(t *testing.T) {
	f, err := New(2, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := f.Allow(nil)
	if !ok {
		t.Fatal("Allow(nil) = false, want true")
	}
	if got != nil {
		t.Fatalf("Allow(nil) = %q, want nil", got)
	}

	got, ok = f.Allow([]byte{})
	if !ok {
		t.Fatal("Allow(empty) = false, want true")
	}
	if got == nil {
		t.Fatal("Allow(empty) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("Allow(empty) = %q, want empty", got)
	}
}

func TestOverflowSliceIsCopied(t *testing.T) {
	overflow := []byte("other")
	f, err := New(1, Aggregate, overflow)
	if err != nil {
		t.Fatal(err)
	}

	overflow[0] = 'X'

	if _, ok := f.Allow([]byte("a")); !ok {
		t.Fatal("first Allow = false, want true")
	}
	got, ok := f.Allow([]byte("b"))
	if !ok {
		t.Fatal("second Allow = false, want true")
	}
	if !bytes.Equal(got, []byte("other")) {
		t.Fatalf("overflow = %q, want %q", got, "other")
	}
}

func TestLimitAndLen(t *testing.T) {
	f, err := New(5, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Limit(); got != 5 {
		t.Fatalf("Limit() = %d, want 5", got)
	}
	if got := f.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestConcurrentAllow(t *testing.T) {
	const limit = 100
	f, err := New(limit, Drop, nil)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f.Allow([]byte{byte(i)})
		}(i)
	}
	wg.Wait()

	if got := f.Len(); got != limit {
		t.Fatalf("Len() = %d, want %d", got, limit)
	}
}
