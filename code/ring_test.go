package main

import (
	"bytes"
	"testing"
	"time"
)

func TestOutputRingReadsByAbsoluteCursor(t *testing.T) {
	ring := newOutputRing(16)
	ring.append([]byte("hello"))

	first := ring.read(0, 3)
	if first.base != 0 || first.next != 3 || !bytes.Equal(first.data, []byte("hel")) {
		t.Fatalf("unexpected first read: %+v", first)
	}
	second := ring.read(first.next, 16)
	if second.next != 5 || !bytes.Equal(second.data, []byte("lo")) {
		t.Fatalf("unexpected second read: %+v", second)
	}
}

func TestOutputRingTruncatesAndClampsStaleCursor(t *testing.T) {
	ring := newOutputRing(5)
	ring.append([]byte("abcdefgh"))

	snapshot := ring.read(0, 16)
	if snapshot.base != 3 || snapshot.next != 8 {
		t.Fatalf("bounds = %d..%d, want 3..8", snapshot.base, snapshot.next)
	}
	if !bytes.Equal(snapshot.data, []byte("defgh")) {
		t.Fatalf("data = %q, want defgh", snapshot.data)
	}
}

func TestOutputRingSignalsWaiters(t *testing.T) {
	ring := newOutputRing(16)
	wait := ring.read(0, 16).changed
	ring.append([]byte("x"))
	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("append did not wake waiter")
	}
}
