package gorestdocs

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
)

// Middleware wraps an http.Handler to record interactions into the given registry.
func Middleware(handler http.Handler, registry *Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request body
		var requestBody []byte
		if r.Body != nil {
			var err error
			requestBody, err = io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(requestBody))
			if err != nil {
				requestBody = nil
			}
		}

		// Record response using httptest.ResponseRecorder
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)

		// Copy recorded response to the actual writer
		result := rec.Result()
		defer result.Body.Close()
		for k, vs := range result.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(result.StatusCode)
		responseBody := rec.Body.Bytes()
		_, _ = w.Write(responseBody)

		// Store the interaction
		registry.Record(Interaction{
			Method:          r.Method,
			Path:            r.URL.Path,
			QueryParams:     r.URL.Query(),
			RequestHeaders:  r.Header.Clone(),
			RequestBody:     requestBody,
			ResponseStatus:  result.StatusCode,
			ResponseHeaders: result.Header.Clone(),
			ResponseBody:    responseBody,
		})
	})
}
