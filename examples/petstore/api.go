package petstore

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Pet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Species string `json:"species"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, []Pet{
				{ID: "1", Name: "Fido", Species: "dog"},
				{ID: "2", Name: "Mittens", Species: "cat"},
			})
		case http.MethodPost:
			var req struct {
				Name    string `json:"name"`
				Species string `json:"species"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}

			writeJSON(w, http.StatusCreated, Pet{
				ID:      "3",
				Name:    req.Name,
				Species: req.Species,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/pets/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/pets/")
		writeJSON(w, http.StatusOK, Pet{
			ID:      id,
			Name:    "Pet " + id,
			Species: "dog",
		})
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"query":   r.URL.Query().Get("q"),
			"results": []Pet{},
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
