package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type RuleManager struct{}

type Rule struct {
	Id          string `json:"id"`
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
	case r.Method == "GET":
		h.getRulesCsv(w)
	default:
		http.Error(w, "invalid request", http.StatusBadRequest)
	}

}

func (h *RuleManager) saveRule(w http.ResponseWriter, r *http.Request) {
	// Read the request body
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

	// Create the CSV reader
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

	// Determine if the first row is header
	var rules []Rule
	startRow := 0

	// Check if first row looks like a header (contains "Project", "Task", etc.)
	firstRow := records[0]
	if len(firstRow) >= 4 &&
		(strings.EqualFold(firstRow[0], "ID") ||
			strings.EqualFold(firstRow[0], "Project") ||
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
			Id:          record[0],
			Project:     record[1],
			Task:        record[2],
			Jira:        record[3],
			Description: record[4],
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

	// Loop rules
	for _, rule := range rules {
		// Check to see if Rule has id, assign if not
		if rule.Id == "" {
			rule.Id = uuid.New().String()
		}

		// See if Rule with this id exists in Weaviate
		_, err := client.Data().ObjectsGetter().
			WithClassName(weaviateClass).
			WithID(rule.Id).
			Do(context.Background())

		ruleExists := true
		if err != nil {
			wce := &fault.WeaviateClientError{}
			if errors.As(err, &wce) && wce.StatusCode == 404 {
				ruleExists = false
			} else {
				fmt.Printf("error getting existing collection: %v\n", err)
				os.Exit(1)
			}

		}

		if ruleExists == false {
			_, err := client.Data().Creator().
				WithID(rule.Id).
				WithClassName(weaviateClass).
				WithProperties(map[string]interface{}{
					"project":     rule.Project,
					"task":        rule.Task,
					"jira":        rule.Jira,
					"description": rule.Description,
				}).
				Do(context.Background())

			if err != nil {
				return false, err
			}
		} else {
			err := client.Data().Updater().
				WithMerge().
				WithID(rule.Id).
				WithClassName(weaviateClass).
				WithProperties(map[string]interface{}{
					"project":     rule.Project,
					"task":        rule.Task,
					"jira":        rule.Jira,
					"description": rule.Description,
				}).
				Do(context.Background())

			if err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func (h *RuleManager) getRulesCsv(w http.ResponseWriter) {
	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		http.Error(w, "error with Weaviate: "+err.Error(), http.StatusInternalServerError)
	}

	rules, err := client.Data().ObjectsGetter().WithClassName(weaviateClass).Do(context.Background())

	log.Printf("got %d rules", len(rules))

	w.WriteHeader(http.StatusOK)

}
