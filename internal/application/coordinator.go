package application

import (
	"context"
	"sync"
)

type lockEntry struct {
	token chan struct{}
	refs  int
}
type Coordinator struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

func NewCoordinator() *Coordinator { return &Coordinator{locks: map[string]*lockEntry{}} }
func (c *Coordinator) Lock(id string) func() {
	unlock, _ := c.LockContext(context.Background(), id)
	return unlock
}

func (c *Coordinator) LockContext(ctx context.Context, id string) (func(), error) {
	c.mu.Lock()
	e := c.locks[id]
	if e == nil {
		e = &lockEntry{token: make(chan struct{}, 1)}
		e.token <- struct{}{}
		c.locks[id] = e
	}
	e.refs++
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.token:
	}
	return func() {
		e.token <- struct{}{}
		c.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(c.locks, id)
		}
		c.mu.Unlock()
	}, nil
}
func (c *Coordinator) Active() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.locks) }
