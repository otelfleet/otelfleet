package contextutil

import (
	"context"
	"sync"

	"github.com/otelfleet/otelcol-lsp/pkg/logutil"
)

type Signal struct {
	name string
	mu   sync.Mutex
	chs  map[chan context.Context]struct{}
}

func NewSignal(name string) *Signal {
	return &Signal{
		name: name,
		chs:  map[chan context.Context]struct{}{},
	}
}

func (s *Signal) Broadcast(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.chs {
		select {
		case ch <- ctx:
		default:
			logutil.From(ctx).With("signal", s.name).Warn("failed to propagate update")
		}
	}
}

// Bind creates a new listening channel bound to the signal. The channel used has a size of 1
// and any given broadcast will signal at least one event, but may signal more than one.
func (s *Signal) Bind() chan context.Context {
	ch := make(chan context.Context, 1)
	s.mu.Lock()
	s.chs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unbind stops the listening channel bound to the signal.
func (s *Signal) Unbind(ch chan context.Context) {
	s.mu.Lock()
	delete(s.chs, ch)
	s.mu.Unlock()

}
