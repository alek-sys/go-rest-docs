package gorestdocs

import (
	"net/http"
	"sync"
)

// Interaction represents a recorded HTTP request/response pair.
type Interaction struct {
	Method          string
	Path            string
	QueryParams     map[string][]string
	RequestHeaders  http.Header
	RequestBody     []byte
	ResponseStatus  int
	ResponseHeaders http.Header
	ResponseBody    []byte
}

// Registry is a thread-safe collection of recorded interactions.
type Registry struct {
	mu           sync.Mutex
	interactions []Interaction
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Record adds an interaction to the registry.
func (r *Registry) Record(i Interaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interactions = append(r.interactions, i)
}

// All returns a copy of all recorded interactions.
func (r *Registry) All() []Interaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Interaction, len(r.interactions))
	copy(result, r.interactions)
	return result
}

// ByPath returns all interactions matching the given path.
func (r *Registry) ByPath(path string) []Interaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []Interaction
	for _, i := range r.interactions {
		if i.Path == path {
			result = append(result, i)
		}
	}
	return result
}

// Reset clears all recorded interactions.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interactions = nil
}
