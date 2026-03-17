package gorestdocs

import (
	"flag"
	"io"
	"net/http"
	"os"
)

var (
	defaultRegistry = NewRegistry()
	defaultPatterns = NewPathPatterns()
	defaultDocs     = NewDocs()
	outputFlag      string
	titleFlag       string
	versionFlag     string
	descriptionFlag string
	sessionFlag     string
)

func init() {
	flag.StringVar(&outputFlag, "gorestdocs.output", "", "file path to write OpenAPI YAML spec (empty = disabled)")
	flag.StringVar(&titleFlag, "gorestdocs.title", "", "API title for the generated spec")
	flag.StringVar(&versionFlag, "gorestdocs.version", "", "API version for the generated spec")
	flag.StringVar(&descriptionFlag, "gorestdocs.description", "", "API description for the generated spec")
	flag.StringVar(&sessionFlag, "gorestdocs.session", "", "directory path to write session recording (empty = disabled)")
}

// DefaultRegistry returns the package-level default registry used by Handler.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// DefaultPatterns returns the package-level default path patterns.
func DefaultPatterns() *PathPatterns {
	return defaultPatterns
}

// RegisterPatterns registers path patterns on the default PathPatterns instance.
// Example: RegisterPatterns("/users/{id}", "/users/{userId}/posts/{postId}")
func RegisterPatterns(patterns ...string) {
	defaultPatterns.Register(patterns...)
}

// Handler wraps an http.Handler with recording middleware using the default registry.
func Handler(h http.Handler) http.Handler {
	return Middleware(h, defaultRegistry)
}

// DefaultDocs returns the package-level default documentation store.
func DefaultDocs() *Docs {
	return defaultDocs
}

// ResetDefaultDocs clears all registered default documentation.
func ResetDefaultDocs() {
	defaultDocs.Reset()
}

// GenerateSpec writes the OpenAPI 3.1 YAML spec from the default registry to w.
func GenerateSpec(w io.Writer, info Info) error {
	spec := BuildSpec(defaultRegistry, info, WithPatterns(defaultPatterns), WithDocs(defaultDocs))
	data, err := MarshalYAML(spec)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// WriteSpecIfEnabled checks the -gorestdocs.output flag and writes the spec file if set.
// Call this from TestMain after tests have run, or from a test cleanup hook.
// Flag-based title, version, and description override the corresponding fields in info
// when they are non-empty.
func WriteSpecIfEnabled(info Info) error {
	if outputFlag == "" {
		return nil
	}

	// Apply flag overrides
	if titleFlag != "" {
		info.Title = titleFlag
	}
	if versionFlag != "" {
		info.Version = versionFlag
	}
	if descriptionFlag != "" {
		info.Description = descriptionFlag
	}

	f, err := os.Create(outputFlag)
	if err != nil {
		return err
	}
	err = GenerateSpec(f, info)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ResetDefaultRegistry clears the default registry. Useful between tests.
func ResetDefaultRegistry() {
	defaultRegistry.Reset()
}

// ResetDefaultPatterns clears all registered default path patterns.
func ResetDefaultPatterns() {
	defaultPatterns.Reset()
}

// WriteSessionIfEnabled checks the -gorestdocs.session flag and writes a session
// recording to the specified directory if the flag is set.
// Call this from TestMain after tests have run.
func WriteSessionIfEnabled() error {
	if sessionFlag == "" {
		return nil
	}
	return WriteSession(sessionFlag, defaultRegistry, defaultPatterns.All())
}
