package main

import (
	"encoding/json"
	"net/http"
)

func RegisterEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("GET /movieAPI/hello", helloHandler)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"message": "Hello from the API!"})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
	}
}
