package runner

import (
	"io"
	"sync"
)

type buffer struct {
	sync.RWMutex
	buf    []byte
	notify func() // Called when data is written to this buffer.
	closed bool   // No more writes accepted when true.
}

func newBuffer(notify func()) *buffer {
	return &buffer{
		notify: notify,
	}
}

// Write implements io.Writer.
func (b *buffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}

	n := len(p)
	if n > 0 {
		b.buf = append(b.buf, p...)
		b.notify()
	}

	return n, nil
}

// Close prevents further writes.
func (b *buffer) Close() {
	b.Lock()
	defer b.Unlock()
	b.closed = true
}

func (b *buffer) String() string {
	b.RLock()
	defer b.RUnlock()
	return string(b.buf)
}
