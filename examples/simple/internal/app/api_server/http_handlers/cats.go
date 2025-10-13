package http_handlers

import (
	"encoding/json"
	"net/http"

	"github.com/cardinalby/examples/simple/internal/app/internal/domain"
)

func RegisterCatsHandlers(
	mux *http.ServeMux,
	catsUsecase domain.CatsUsecase,
) {
	mux.HandleFunc("GET /cats", func(w http.ResponseWriter, r *http.Request) {
		cats, err := catsUsecase.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Failed to get cats: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if cats == nil {
			cats = []domain.Cat{}
		}
		if err := json.NewEncoder(w).Encode(cats); err != nil {
			http.Error(w, "Failed to encode cats: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("POST /cats", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
			Age  uint   `json:"age"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to decode request: "+err.Error(), http.StatusBadRequest)
			return
		}
		cat, err := catsUsecase.Add(r.Context(), req.Name, req.Age)
		if err != nil {
			http.Error(w, "Failed to add cat: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(cat); err != nil {
			http.Error(w, "Failed to encode cat: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
