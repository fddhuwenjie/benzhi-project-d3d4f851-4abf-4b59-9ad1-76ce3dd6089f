package application

import "sync"

type lockEntry struct {
	mu   sync.Mutex
	refs int
}
type Coordinator struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

func NewCoordinator() *Coordinator { return &Coordinator{locks: map[string]*lockEntry{}} }
func (c *Coordinator) Lock(id string) func() {
	c.mu.Lock()
	e := c.locks[id]
	if e == nil {
		e = &lockEntry{}
		c.locks[id] = e
	}
	e.refs++
	c.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		c.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(c.locks, id)
		}
		c.mu.Unlock()
	}
}
func (c *Coordinator) Active() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.locks) }
