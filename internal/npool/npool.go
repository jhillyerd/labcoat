package npool

import (
	"context"
	"fmt"
)

type Pool struct {
	avail chan Worker
}

// New constructs a new worker pool of the specified size.
func New(name string, size int) *Pool {
	c := make(chan Worker, size)
	p := &Pool{avail: c}

	for i := 0; i < size; i++ {
		w := Worker{
			id:   fmt.Sprintf("%s:%d", name, i+1),
			pool: p,
		}
		c <- w
	}

	return p
}

func (p *Pool) Get(ctx context.Context) (*Worker, error) {
	select {
	case w := <-p.avail:
		return &w, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Worker represents the permission to do work.
type Worker struct {
	id   string
	pool *Pool
}

// Done returns this worker to the pool.
func (w Worker) Done() {
	w.pool.avail <- w
}

// String returns the name & ID of this worker.
func (w Worker) String() string {
	return w.id
}
