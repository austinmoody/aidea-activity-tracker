package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"net/http"
	"time"
)

type ActivityManager struct{}

func (h *ActivityManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == "POST":
		h.saveActivity(w, r)
	}
}

func (h *ActivityManager) saveActivity(w http.ResponseWriter, r *http.Request) {

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON request
	var request Activity
	err = json.Unmarshal(body, &request)
	if err != nil {
		http.Error(w, "Error parsing JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set created at time & activity id
	request.CreatedAt = time.Now()
	request.ActivityId = uuid.New().String()

	saveActivityCsv(request)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(request)

}
