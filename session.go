package gorestdocs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const sessionVersion = "1"

// SessionManifest is written as session.json in the session directory.
type SessionManifest struct {
	Title            string    `json:"title"`
	Version          string    `json:"version"`
	RecordedAt       time.Time `json:"recorded_at"`
	Patterns         []string  `json:"patterns"`
	InteractionCount int       `json:"interaction_count"`
}

// sessionInteraction is the on-disk JSON representation of an Interaction.
type sessionInteraction struct {
	Method          string              `json:"method"`
	Path            string              `json:"path"`
	QueryParams     map[string][]string `json:"query_params,omitempty"`
	RequestHeaders  map[string][]string `json:"request_headers,omitempty"`
	RequestBody     []byte              `json:"request_body,omitempty"`
	ResponseStatus  int                 `json:"response_status"`
	ResponseHeaders map[string][]string `json:"response_headers,omitempty"`
	ResponseBody    []byte              `json:"response_body,omitempty"`
}

func interactionToRecord(i Interaction) sessionInteraction {
	return sessionInteraction{
		Method:          i.Method,
		Path:            i.Path,
		QueryParams:     mapHeader(i.QueryParams),
		RequestHeaders:  mapHeader(i.RequestHeaders),
		RequestBody:     i.RequestBody,
		ResponseStatus:  i.ResponseStatus,
		ResponseHeaders: mapHeader(i.ResponseHeaders),
		ResponseBody:    i.ResponseBody,
	}
}

func recordToInteraction(r sessionInteraction) Interaction {
	return Interaction{
		Method:          r.Method,
		Path:            r.Path,
		QueryParams:     r.QueryParams,
		RequestHeaders:  http.Header(r.RequestHeaders),
		RequestBody:     r.RequestBody,
		ResponseStatus:  r.ResponseStatus,
		ResponseHeaders: http.Header(r.ResponseHeaders),
		ResponseBody:    r.ResponseBody,
	}
}

// mapHeader converts an http.Header (or similar map) to a plain map for JSON serialization.
func mapHeader(h map[string][]string) map[string][]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

// WriteSession writes all interactions from registry to dir as a session directory.
// dir is created if it does not exist. Any existing interaction files are removed
// before writing so that a second call to WriteSession on the same directory does
// not leave stale files from a previous run. patterns lists the registered path patterns.
func WriteSession(dir string, registry *Registry, patterns []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// Remove existing interaction files so a repeated WriteSession doesn't merge
	// old and new interactions.
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "session.json" {
				continue
			}
			if filepath.Ext(entry.Name()) == ".json" {
				_ = os.Remove(filepath.Join(dir, entry.Name()))
			}
		}
	}

	interactions := registry.All()

	manifest := SessionManifest{
		Title:            "recorded session",
		Version:          sessionVersion,
		RecordedAt:       time.Now().UTC(),
		Patterns:         patterns,
		InteractionCount: len(interactions),
	}

	if err := writeJSON(filepath.Join(dir, "session.json"), manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	for idx, interaction := range interactions {
		name := fmt.Sprintf("%03d_%s_%s.json", idx+1, interaction.Method, sanitizePath(interaction.Path))
		rec := interactionToRecord(interaction)
		if err := writeJSON(filepath.Join(dir, name), rec); err != nil {
			return fmt.Errorf("write interaction %d: %w", idx+1, err)
		}
	}

	return nil
}

// ReadSession loads a session directory written by WriteSession.
// Returns the manifest and all recorded interactions.
func ReadSession(dir string) (SessionManifest, []Interaction, error) {
	var manifest SessionManifest
	if err := readJSON(filepath.Join(dir, "session.json"), &manifest); err != nil {
		return manifest, nil, fmt.Errorf("read manifest: %w", err)
	}

	interactions := make([]Interaction, 0, manifest.InteractionCount)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return manifest, nil, fmt.Errorf("read session dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "session.json" {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var rec sessionInteraction
		if err := readJSON(filepath.Join(dir, entry.Name()), &rec); err != nil {
			return manifest, nil, fmt.Errorf("read interaction %s: %w", entry.Name(), err)
		}
		interactions = append(interactions, recordToInteraction(rec))
	}

	if len(interactions) != manifest.InteractionCount {
		return manifest, interactions, fmt.Errorf("session corrupt: manifest claims %d interaction(s), found %d file(s)", manifest.InteractionCount, len(interactions))
	}

	return manifest, interactions, nil
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	werr := enc.Encode(v)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(path)
		return werr
	}
	return cerr
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(v)
}

// sanitizePath converts a URL path to a safe filename segment.
func sanitizePath(path string) string {
	out := make([]byte, 0, len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' {
			if len(out) > 0 {
				out = append(out, '_')
			}
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "root"
	}
	return string(out)
}
