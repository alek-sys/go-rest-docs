package gorestdocs

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ReplayServer is an HTTP server that replays recorded interactions.
type ReplayServer struct {
	manifest     SessionManifest
	interactions []Interaction
	patterns     *PathPatterns
	// grouped maps "METHOD pattern" -> []Interaction
	grouped map[string][]Interaction
}

// NewReplayServer loads a session directory and prepares a replay server.
func NewReplayServer(dir string) (*ReplayServer, error) {
	manifest, interactions, err := ReadSession(dir)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	pp := NewPathPatterns()
	if len(manifest.Patterns) > 0 {
		pp.Register(manifest.Patterns...)
	}

	s := &ReplayServer{
		manifest:     manifest,
		interactions: interactions,
		patterns:     pp,
		grouped:      make(map[string][]Interaction),
	}
	s.buildIndex()
	return s, nil
}

// buildIndex groups interactions by "METHOD pattern" for fast lookup.
func (s *ReplayServer) buildIndex() {
	for _, i := range s.interactions {
		pattern := s.resolvePattern(i.Path)
		key := i.Method + " " + pattern
		s.grouped[key] = append(s.grouped[key], i)
	}
}

// resolvePattern returns the registered pattern for a path, or the path itself.
func (s *ReplayServer) resolvePattern(path string) string {
	if pattern, ok := s.patterns.Match(path); ok {
		return pattern
	}
	return path
}

// Endpoints returns a sorted list of "METHOD pattern" strings for all recorded endpoints.
func (s *ReplayServer) Endpoints() []string {
	endpoints := make([]string, 0, len(s.grouped))
	for k := range s.grouped {
		endpoints = append(endpoints, k)
	}
	sort.Strings(endpoints)
	return endpoints
}

// Handler returns an http.Handler that replays recorded responses.
func (s *ReplayServer) Handler() http.Handler {
	return http.HandlerFunc(s.ServeHTTP)
}

// ServeHTTP matches the request to a recorded interaction and replays it.
func (s *ReplayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pattern := s.resolvePattern(r.URL.Path)
	key := r.Method + " " + pattern

	candidates, ok := s.grouped[key]
	if !ok {
		s.notFound(w, r)
		return
	}

	interaction := s.pick(candidates, r)

	for k, vals := range interaction.ResponseHeaders {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	status := interaction.ResponseStatus
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(interaction.ResponseBody)
}

// pick selects the best matching interaction from candidates.
// Filters to exact path matches first (if any), then returns the candidate
// with the most matching query parameters. Falls back to the first candidate.
func (s *ReplayServer) pick(candidates []Interaction, r *http.Request) Interaction {
	// Narrow to exact path matches if any exist.
	pool := candidates
	var exactMatches []Interaction
	for _, c := range candidates {
		if c.Path == r.URL.Path {
			exactMatches = append(exactMatches, c)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0]
	}
	if len(exactMatches) > 1 {
		pool = exactMatches
	}

	// Score remaining candidates by query parameter overlap.
	// Matching params add to the score; extra params on the candidate that the
	// request does not have subtract from it. This ensures a no-param candidate
	// is preferred over a param-carrying one when the request has no params.
	best := pool[0]
	bestScore := -len(pool[0].QueryParams) - 1
	reqQuery := r.URL.Query()
	for _, c := range pool {
		score := 0
		for k, vals := range c.QueryParams {
			reqVals := reqQuery[k]
			matched := 0
			for i, v := range vals {
				if i < len(reqVals) && reqVals[i] == v {
					matched++
				}
			}
			if matched > 0 {
				score += matched
			} else {
				score-- // penalize params the request doesn't have
			}
		}
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

// notFound writes a 404 response listing available endpoints.
func (s *ReplayServer) notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	endpoints := s.Endpoints()
	var sb strings.Builder
	fmt.Fprintf(&sb, "no recorded interaction for %s %s\n\navailable endpoints:\n", r.Method, r.URL.Path)
	for _, ep := range endpoints {
		fmt.Fprintf(&sb, "  %s\n", ep)
	}
	_, _ = w.Write([]byte(sb.String()))
}
