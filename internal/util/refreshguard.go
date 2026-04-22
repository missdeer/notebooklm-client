package util

import "sync"

type RefreshGuard struct {
	mu      sync.Mutex
	running bool
	done    chan struct{}
	err     error
}

func NewRefreshGuard() *RefreshGuard {
	return &RefreshGuard{}
}

// Do runs fn if no refresh is in-flight, otherwise waits for the existing one.
func (g *RefreshGuard) Do(fn func() error) error {
	g.mu.Lock()
	if g.running {
		done := g.done
		g.mu.Unlock()
		<-done
		g.mu.Lock()
		err := g.err
		g.mu.Unlock()
		if err == nil {
			return nil
		}
		// Previous attempt failed — fall through to start our own.
		g.mu.Lock()
	}

	g.running = true
	g.done = make(chan struct{})
	g.mu.Unlock()

	err := fn()

	g.mu.Lock()
	g.err = err
	g.running = false
	close(g.done)
	g.mu.Unlock()

	return err
}
