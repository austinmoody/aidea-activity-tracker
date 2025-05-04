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
	recategorizeById *regexp.Regexp
)

type ActivityManager struct{}

func init() {
	activityTodayCsv = regexp.MustCompile(`^/api/v1/activity/today$`)
	activityById = regexp.MustCompile(`^/api/v1/activity/([0-9a-f-]+)$`)
	activityByDateId = regexp.MustCompile(`^/api/v1/activity/([0-9]{8})/([0-9a-f-]+)$`)
	recategorizeById = regexp.MustCompile(`^/api/v1/activity/recategorize/([0-9a-f-]+)$`)
}

func (h *ActivityManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case
		r.Method == "POST":
		h.saveActivity(w, r)
	case
		r.Method == "GET" && activityTodayCsv.MatchString(r.URL.String()):
		h.getTodayCsv(w)
	case
		r.Method == "GET" && activityById.MatchString(r.URL.String()):
		h.getActivityById(w, r)
	case
		r.Method == "GET" && activityByDateId.MatchString(r.URL.String()):
		h.getActivityByDateId(w, r)
	case
		r.Method == "PATCH" && recategorizeById.MatchString(r.URL.String()):
		h.recategorizeActivity(w, r)
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

func (h *ActivityManager) recategorizeActivity(w http.ResponseWriter, r *http.Request) {
	// Extract activity ID from URL using the regex pattern
	matches := recategorizeById.FindStringSubmatch(r.URL.String())
	if len(matches) < 2 {
		http.Error(w, "Invalid activity ID in URL", http.StatusBadRequest)
		return
	}
	activityId := matches[1]

	log.Printf("activity to REcategorize: %s\n", activityId)

	// Generate today's filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	activity, err := getActivityInFileById(activityId, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (Activity{} == activity) {
		http.Error(w, "activity not found", http.StatusNotFound)
		return
	}

	// TODO - this function and the saveActivity could be refactored, shared logic
	// Have Ollama determine Jira/Tempo formatted duration
	// from the user's input
	duration, err := getDuration(activity)
	if err != nil {
		http.Error(w, "Error obtaining duration from input: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("\tollma extracted duration: %s\n", duration)
	activity.Duration = duration

	activity = categorizeActivity(activity)
	log.Printf("\tweaviate REcategorized as Project: %s\n", activity.Project)
	log.Printf("\tweaviate REcategorized as Task: %s\n", activity.Task)
	log.Printf("\tweaviate REcategorized as Jira: %s\n", activity.Jira)
	log.Printf("\tweaviate REcategorization grade: %s\n", activity.CategorizationGrade)

	// Update the activity in the CSV file
	err = updateActivityInCSV(activity, filename)
	if err != nil {
		http.Error(w, "Error updating activity in CSV: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(activity)
}

func (h *ActivityManager) getTodayCsv(w http.ResponseWriter) {
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

func (h *ActivityManager) getActivityByDateId(w http.ResponseWriter, r *http.Request) {
	// Extract activity ID from URL using the regex pattern
	matches := activityByDateId.FindStringSubmatch(r.URL.String())
	if len(matches) < 2 {
		http.Error(w, "Invalid activity ID in URL", http.StatusBadRequest)
		return
	}
	fileDate := matches[1]
	activityId := matches[2]

	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", fileDate)

	activity, err := getActivityInFileById(activityId, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (Activity{} == activity) {
		http.Error(w, "activity not found", http.StatusNotFound)
		return
	}

	// Return the activity as JSON
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(activity)
	return
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

	activity, err := getActivityInFileById(activityId, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (Activity{} == activity) {
		http.Error(w, "activity not found", http.StatusNotFound)
		return
	}

	// Return the activity as JSON
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(activity)
	return
}

func getActivityInFileById(activityId string, filename string) (Activity, error) {
	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return Activity{}, fmt.Errorf("no activity data file '%s' found'", filename)
	}

	// Open the CSV file
	file, err := os.Open(filename)
	if err != nil {
		return Activity{}, fmt.Errorf("error opening csv file: %v", err)
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(file)

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return Activity{}, fmt.Errorf("error reading csv headers: %v", err)
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
			return Activity{}, fmt.Errorf("error reading csv record: %v", err)
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
								// First try our simplified format (what we're writing to CSV now)
								t, err := time.Parse("2006-01-02 15:04:05", record[idx])
								if err != nil {
									// Try RFC3339 format
									t, err = time.Parse(time.RFC3339, record[idx])
									if err != nil {
										// Try the original verbose format for backward compatibility
										t, err = time.Parse("2006-01-02 15:04:05.999999 -0700 MST m=+0.000000000", record[idx])
										if err != nil {
											log.Printf("Error parsing time from '%s': %v", record[idx], err)
										}
									}
								}
								field.Set(reflect.ValueOf(t))
							}
						}
					}
				}
			}
			return activity, nil
		}
	}

	return Activity{}, fmt.Errorf("activity not found")
}

// updateActivityInCSV replaces a specific activity in the CSV file
func updateActivityInCSV(activity Activity, filename string) error {
	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("no activity data file '%s' found", filename)
	}

	// Read the entire CSV file into memory
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening csv file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading csv file: %v", err)
	}

	// Get headers from the first row
	headers := records[0]
	headerIndex := make(map[string]int)
	for i, header := range headers {
		headerIndex[header] = i
	}

	// Find the activity ID index
	activityIdIndex, exists := headerIndex["ActivityId"]
	if !exists {
		return fmt.Errorf("ActivityId column not found in CSV")
	}

	// Find the row with the matching activity ID
	rowIndex := -1
	for i, record := range records {
		if i == 0 { // Skip the header row
			continue
		}
		if record[activityIdIndex] == activity.ActivityId {
			rowIndex = i
			break
		}
	}

	if rowIndex == -1 {
		return fmt.Errorf("activity with ID %s not found in CSV", activity.ActivityId)
	}

	// Replace the row with the updated activity values
	records[rowIndex] = getActivitySlice(activity)

	// Write the updated records back to the file
	file.Close() // Close before writing to avoid issues
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file for writing: %v", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	err = writer.WriteAll(records)
	if err != nil {
		return fmt.Errorf("error writing updated records to CSV: %v", err)
	}

	log.Printf("Successfully updated activity %s in CSV file", activity.ActivityId)
	return nil
}

// TODO - a function to trigger categorization of any today where Categorized = false
