package main

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Message string `json:"message"`
}

type LoginResponse struct {
	LoggedIn bool   `json:"loggedIn"`
	UserID   string `json:"userId,omitempty"`
	
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
	}
}
