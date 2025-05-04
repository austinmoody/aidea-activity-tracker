package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"time"
)

var (
	activityTodayCsv *regexp.Regexp
	activityById     *regexp.Regexp
	activityByDateId *regexp.Regexp
)

type ActivityManager struct{}

func init() {
	activityTodayCsv = regexp.MustCompile(`^/api/v1/activity/today$`)
	activityById = regexp.MustCompile(`^/api/v1/activity/([0-9a-f-]+)$`)
	activityByDateId = regexp.MustCompile(`^/api/v1/activity/[0-9]{8}/([0-9a-f-]+)$`)
}

func (h *ActivityManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case
		r.Method == "POST":
		h.saveActivity(w, r)
	case
		r.Method == "GET" && activityTodayCsv.MatchString(r.URL.String()):
		h.getTodayCsv(w, r)
	case
		r.Method == "GET" && activityById.MatchString(r.URL.String()):
		h.getActivityById(w, r)
	default:
		http.Error(w, "invalid request", http.StatusBadRequest)
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
	request.Categorized = false

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

func (h *ActivityManager) getTodayCsv(w http.ResponseWriter, r *http.Request) {
	// Generate today's filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		http.Error(w, "No activity data for today", http.StatusNotFound)
		return
	}

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, "Error opening CSV file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set response headers for CSV file download
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	// Copy the file contents to the response
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Error sending CSV file: %v", err)
	}
}

func (h *ActivityManager) getActivityById(w http.ResponseWriter, r *http.Request) {
	// Extract activity ID from URL using the regex pattern
	matches := activityById.FindStringSubmatch(r.URL.String())
	if len(matches) < 2 {
		http.Error(w, "Invalid activity ID in URL", http.StatusBadRequest)
		return
	}
	activityId := matches[1]

	// Generate today's filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		http.Error(w, "No activity data for today", http.StatusNotFound)
		return
	}

	// Open the CSV file
	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, "Error opening CSV file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(file)

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		http.Error(w, "Error reading CSV headers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a map to store header indices for easy lookup
	headerIndex := make(map[string]int)
	for i, header := range headers {
		headerIndex[header] = i
	}

	// Read all records and find the one with matching activity ID
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading CSV record: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if this record's ActivityId matches the requested one
		if record[headerIndex["ActivityId"]] == activityId {
			// Create an Activity struct from the CSV record
			activity := Activity{}
			activityValue := reflect.ValueOf(&activity).Elem()
			activityType := reflect.TypeOf(activity)

			// Map CSV values to struct fields
			for i := 0; i < activityType.NumField(); i++ {
				fieldName := activityType.Field(i).Name
				if idx, exists := headerIndex[fieldName]; exists && idx < len(record) {
					field := activityValue.FieldByName(fieldName)
					if field.CanSet() {
						// Set value based on field type
						switch field.Kind() {
						case reflect.String:
							field.SetString(record[idx])
						case reflect.Float64:
							val, _ := strconv.ParseFloat(record[idx], 64)
							field.SetFloat(val)
						case reflect.Bool:
							val, _ := strconv.ParseBool(record[idx])
							field.SetBool(val)
						case reflect.Struct:
							// Handle time.Time
							if field.Type() == reflect.TypeOf(time.Time{}) {
								t, _ := time.Parse(time.RFC3339, record[idx])
								field.Set(reflect.ValueOf(t))
							}
						}
					}
				}
			}

			// Return the activity as JSON
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(activity)
			return
		}
	}

	// If we get here, no matching activity was found
	http.Error(w, "Activity not found", http.StatusNotFound)
}

// TODO - a function to get an activity by id - Do I need this?
// TODO - a function to trigger categorization of any today where Categorized = false
// TODO - a function to recategorize a specific activity by id
