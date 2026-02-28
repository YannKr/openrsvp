package notification

import (
	"fmt"
	"sync"
)

// Registry manages notification providers per channel.
type Registry struct {
	mu        sync.RWMutex
	providers map[Channel]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[Channel]Provider),
	}
}

// Register adds a provider for a channel. Replaces existing provider for the same channel.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Channel()] = p
}

// Get returns the provider for a channel.
func (r *Registry) Get(ch Channel) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[ch]
	if !ok {
		return nil, fmt.Errorf("no provider registered for channel: %s", ch)
	}
	return p, nil
}

// Has checks if a provider is registered for a channel.
func (r *Registry) Has(ch Channel) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[ch]
	return ok
}

// Channels returns all channels that have providers.
func (r *Registry) Channels() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	channels := make([]Channel, 0, len(r.providers))
	for ch := range r.providers {
		channels = append(channels, ch)
	}
	return channels
}
