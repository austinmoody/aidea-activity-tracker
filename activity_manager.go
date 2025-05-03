package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"log"
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

	log.Println("activity to save received")

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request as Activity
	var request Activity
	err = json.Unmarshal(body, &request)
	if err != nil {
		http.Error(w, "error parsing input: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("\tactivity description: %s\n", request.InputDescription)

	// Set created at time & activity id
	request.CreatedAt = time.Now()
	request.ActivityId = uuid.New().String()

	log.Printf("\tassigned id %s\n", request.ActivityId)

	// Have Ollama determine Jira/Tempo formatted duration
	// from the user's input
	duration, err := getDuration(request)
	if err != nil {
		http.Error(w, "Error obtaining duration from input: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("\tollma extracted duration: %s\n", duration)
	request.Duration = duration

	request = categorizeActivity(request)
	log.Printf("\tweaviate categorized as Project: %s\n", request.Project)
	log.Printf("\tweaviate categorized as Task: %s\n", request.Task)
	log.Printf("\tweaviate categorized as Jira: %s\n", request.Jira)
	log.Printf("\tweaviate categorization grade: %s\n", request.CategorizationGrade)

	saveActivityCsv(request)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(request)

}
