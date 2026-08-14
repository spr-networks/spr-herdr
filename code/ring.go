package main

import "sync"

type outputRing struct {
	mu      sync.Mutex
	base    uint64
	data    []byte
	limit   int
	changed chan struct{}
}

func newOutputRing(limit int) *outputRing {
	return &outputRing{
		limit:   limit,
		changed: make(chan struct{}),
	}
}

func (ring *outputRing) append(data []byte) {
	if len(data) == 0 {
		return
	}

	ring.mu.Lock()
	defer ring.mu.Unlock()

	if len(data) >= ring.limit {
		ring.base += uint64(len(ring.data) + len(data) - ring.limit)
		ring.data = append(ring.data[:0], data[len(data)-ring.limit:]...)
	} else {
		ring.data = append(ring.data, data...)
		if overflow := len(ring.data) - ring.limit; overflow > 0 {
			ring.base += uint64(overflow)
			copy(ring.data, ring.data[overflow:])
			ring.data = ring.data[:len(ring.data)-overflow]
		}
	}

	close(ring.changed)
	ring.changed = make(chan struct{})
}

type outputSnapshot struct {
	base    uint64
	next    uint64
	data    []byte
	changed <-chan struct{}
	stale   bool
}

func (ring *outputRing) read(cursor uint64, maximum int) outputSnapshot {
	ring.mu.Lock()
	defer ring.mu.Unlock()

	end := ring.base + uint64(len(ring.data))
	if cursor < ring.base {
		return outputSnapshot{
			base:    ring.base,
			next:    end,
			changed: ring.changed,
			stale:   true,
		}
	}
	if cursor > end {
		cursor = end
	}

	offset := int(cursor - ring.base)
	length := len(ring.data) - offset
	if length > maximum {
		length = maximum
	}
	data := append([]byte(nil), ring.data[offset:offset+length]...)

	return outputSnapshot{
		base:    ring.base,
		next:    cursor + uint64(length),
		data:    data,
		changed: ring.changed,
	}
}

func (ring *outputRing) bounds() (uint64, uint64) {
	ring.mu.Lock()
	defer ring.mu.Unlock()
	return ring.base, ring.base + uint64(len(ring.data))
}
