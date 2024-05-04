package runner

import "sync"

type buffer struct {
	sync.RWMutex
	buf    []byte
	notify func() // Called when data is written to this buffer.
}

func newBuffer(notify func()) *buffer {
	return &buffer{
		notify: notify,
	}
}

// Write implements io.Writer.
func (b *buffer) Write(p []byte) (int, error) {
	n := len(p)
	if n > 0 {
		b.Lock()
		defer b.Unlock()
		b.buf = append(b.buf, p...)
		b.notify()
	}

	return n, nil
}

func (b *buffer) String() string {
	b.RLock()
	defer b.RUnlock()
	return string(b.buf)
}
