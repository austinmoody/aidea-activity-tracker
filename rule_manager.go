package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
	"io"
	"log"
	"net/http"
	"strings"
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
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "text/csv") {
			h.saveCsvRules(w, r)
		} else if contentType == "application/json" {
			h.saveRule(w, r)
		} else {
			http.Error(w, "Content-Type must be application/json or text/csv", http.StatusUnsupportedMediaType)
		}
	}

}

func (h *RuleManager) saveRule(w http.ResponseWriter, r *http.Request) {
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

	// Convert single rule to slice for batch processing
	rules := []Rule{rule}
	success, err := saveRulesToWeaviate(rules)
	if err != nil {
		http.Error(w, "Error saving rule to Weaviate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !success {
		http.Error(w, "Failed to save rule to Weaviate", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

func (h *RuleManager) saveCsvRules(w http.ResponseWriter, r *http.Request) {
	// Read the body first to detect delimiter
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Convert to string for analysis
	bodyStr := string(bodyBytes)

	// Create CSV reader
	reader := csv.NewReader(strings.NewReader(bodyStr))

	// Auto-detect delimiter by checking first line
	firstLine := strings.Split(bodyStr, "\n")[0]

	// Count occurrences of tabs and commas in the first line
	tabCount := strings.Count(firstLine, "\t")
	commaCount := strings.Count(firstLine, ",")

	// If more tabs than commas, use tab delimiter
	if tabCount > 0 && (tabCount > commaCount || commaCount == 0) {
		reader.Comma = '\t'
		log.Printf("Detected tab-delimited CSV (tabs: %d, commas: %d)", tabCount, commaCount)
	} else {
		// Default is comma-delimited
		log.Printf("Using comma-delimited CSV (tabs: %d, commas: %d)", tabCount, commaCount)
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "Error parsing CSV: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(records) < 1 {
		http.Error(w, "CSV file is empty", http.StatusBadRequest)
		return
	}

	// Determine if first row is header
	var rules []Rule
	startRow := 0

	// Check if first row looks like a header (contains "Project", "Task", etc.)
	firstRow := records[0]
	if len(firstRow) >= 4 &&
		(strings.EqualFold(firstRow[0], "Project") ||
			strings.EqualFold(firstRow[1], "Task") ||
			strings.EqualFold(firstRow[2], "Jira") ||
			strings.EqualFold(firstRow[3], "Description")) {
		startRow = 1
		log.Printf("CSV contains header row, processing from row 2")
	}

	// Process data rows
	for i := startRow; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			log.Printf("Warning: Row %d has fewer than 4 columns, skipping", i+1)
			continue
		}

		rule := Rule{
			Project:     record[0],
			Task:        record[1],
			Jira:        record[2],
			Description: record[3],
		}
		rules = append(rules, rule)
	}

	if len(rules) == 0 {
		http.Error(w, "No valid rules found in CSV", http.StatusBadRequest)
		return
	}

	// Save rules to Weaviate
	log.Printf("Processing %d rules from CSV", len(rules))
	success, err := saveRulesToWeaviate(rules)
	if err != nil {
		http.Error(w, "Error saving rules to Weaviate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !success {
		http.Error(w, "Failed to save rules to Weaviate", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("Successfully processed %d rules", len(rules)),
		"count":   len(rules),
	})
}

func saveRulesToWeaviate(rules []Rule) (bool, error) {
	// Create Weaviate client
	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		return false, err
	}

	// Convert rules to Weaviate objects
	objects := make([]*models.Object, len(rules))
	for i, rule := range rules {
		objects[i] = &models.Object{
			Class: weaviateClass,
			Properties: map[string]any{
				"project":     rule.Project,
				"task":        rule.Task,
				"jira":        rule.Jira,
				"description": rule.Description,
			},
		}
	}

	// Batch write to Weaviate
	batchResult, err := client.Batch().ObjectsBatcher().WithObjects(objects...).Do(context.Background())
	if err != nil {
		return false, err
	}

	// Check for errors in results
	for _, res := range batchResult {
		if res.Result.Errors != nil {
			return false, fmt.Errorf("batch operation failed: %v", res.Result.Errors)
		}
	}

	return true, nil
}
