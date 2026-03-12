package gorestdocs

import (
	"flag"
	"io"
	"net/http"
	"os"
)

var (
	defaultRegistry = NewRegistry()
	outputFlag      string
)

func init() {
	flag.StringVar(&outputFlag, "gorestdocs.output", "", "file path to write OpenAPI YAML spec (empty = disabled)")
}

// DefaultRegistry returns the package-level default registry used by Handler.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Handler wraps an http.Handler with recording middleware using the default registry.
func Handler(h http.Handler) http.Handler {
	return Middleware(h, defaultRegistry)
}

// GenerateSpec writes the OpenAPI 3.1 YAML spec from the default registry to w.
func GenerateSpec(w io.Writer, info Info) error {
	spec := BuildSpec(defaultRegistry, info)
	data, err := MarshalYAML(spec)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// WriteSpecIfEnabled checks the -gorestdocs.output flag and writes the spec file if set.
// Call this from TestMain after tests have run, or from a test cleanup hook.
func WriteSpecIfEnabled(info Info) error {
	if outputFlag == "" {
		return nil
	}
	f, err := os.Create(outputFlag)
	if err != nil {
		return err
	}
	defer f.Close()
	return GenerateSpec(f, info)
}

// ResetDefaultRegistry clears the default registry. Useful between tests.
func ResetDefaultRegistry() {
	defaultRegistry.Reset()
}
