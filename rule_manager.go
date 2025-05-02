package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
	"io"
	"net/http"
)

type RuleManager struct{}

type Rule struct {
	Project     string `json:"project"`
	Task        string `json:"task"`
	Jira        string `json:"jira"`
	Description string `json:"description"`
}

func (h *RuleManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == "POST":
		h.saveRule(w, r)
	}

}

func (h *RuleManager) saveRule(w http.ResponseWriter, r *http.Request) {
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
	var rule Rule
	err = json.Unmarshal(body, &rule)
	if err != nil {
		http.Error(w, "Error parsing JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// convert rule into a models.Object for Weaviate
	// TODO - must be a more elegant way to do this
	object := &models.Object{
		Class: "ActivityRules",
		Properties: map[string]any{
			"project":     rule.Project,
			"task":        rule.Task,
			"jira":        rule.Jira,
			"description": rule.Description,
		},
	}

	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		fmt.Println(err)
	}

	batchResult, err := client.Batch().ObjectsBatcher().WithObjects(object).Do(context.Background())
	if err != nil {
		// TODO - seriously don't panic here, return some kind of error message
		w.WriteHeader(http.StatusBadRequest)
		panic(err)
	}

	for _, res := range batchResult {
		if res.Result.Errors != nil {
			panic(res.Result.Errors.Error)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)

}
