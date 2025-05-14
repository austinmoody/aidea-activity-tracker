package main

import (
	"bytes"
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
	activityTodayCsv  *regexp.Regexp
	activityCsvByDate *regexp.Regexp
	activityById      *regexp.Regexp
	activityByDateId  *regexp.Regexp
	recategorizeById  *regexp.Regexp
	activityToTempo   *regexp.Regexp
)

type ActivityManager struct{}

type JiraTempoPayload struct {
	IssueKey         string `json:"issueKey"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	StartDate        string `json:"startDate"`
	Description      string `json:"description"`
}

func init() {
	activityTodayCsv = regexp.MustCompile(`^/api/v1/activity/csv/today$`)
	activityCsvByDate = regexp.MustCompile(`^/api/v1/activity/csv/([0-9]{8})$`)
	activityById = regexp.MustCompile(`^/api/v1/activity/([0-9a-f-]+)$`)
	activityByDateId = regexp.MustCompile(`^/api/v1/activity/([0-9]{8})/([0-9a-f-]+)$`)
	recategorizeById = regexp.MustCompile(`^/api/v1/activity/recategorize/([0-9a-f-]+)$`)
	activityToTempo = regexp.MustCompile(`^/api/v1/activity/tempo/([0-9]{8})/([0-9a-f-]+)$`)
}

func (h *ActivityManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Printf("activity manager - %s %s", r.Method, r.RequestURI)

	switch {
	case
		r.Method == "POST" && activityToTempo.MatchString(r.URL.String()):
		h.activityToTempoById(w, r)
	case
		r.Method == "POST":
		h.saveActivity(w, r)
	case
		r.Method == "GET" && activityTodayCsv.MatchString(r.URL.String()):
		h.getTodayCsv(w)
	case
		r.Method == "GET" && activityCsvByDate.MatchString(r.URL.String()):
		h.getCsvByDate(w, r)
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

// @Summary Create a new activity
// @Description Create a new activity with the provided details
// @Tags activity
// @Accept json
// @Produce json
// @Param activity body Activity true "Activity object to be created"
// @Success 201 {object} Activity "Successfully created activity"
// @Failure 400 {object} string "Bad request"
// @Failure 415 {object} string "Unsupported media type"
// @Router /activity [post]
func (h *ActivityManager) saveActivity(w http.ResponseWriter, r *http.Request) {

	log.Println("activity manager - activity to save received")

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Printf("\tinvalid content type: %s", contentType)
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
	request.PostedToJiraTempo = false

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

	log.Println("\tCSV entry saved")

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(request)

}

// @Summary Recategorize an activity
// @Description Recategorize an existing activity by its ID
// @Tags activity
// @Accept json
// @Produce json
// @Param id path string true "Activity ID"
// @Success 200 {object} Activity "Successfully recategorized activity"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "Activity not found"
// @Router /activity/recategorize/{id} [patch]
func (h *ActivityManager) recategorizeActivity(w http.ResponseWriter, r *http.Request) {

	log.Println("activity manager - activity to recategorize received")

	// Extract activity ID from URL using the regex pattern
	matches := recategorizeById.FindStringSubmatch(r.URL.String())
	if len(matches) < 1 {
		log.Printf("\tinvalid id received in URL")
		http.Error(w, "invalid activity ID in URL", http.StatusBadRequest)
		return
	}
	activityId := matches[1]

	log.Printf("activity manager - REcategorize ID: %s\n", activityId)

	// Generate today's filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	activity, err := getActivityInFileById(activityId, filename)
	if err != nil {
		log.Printf("\tunable to get activity from file: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (Activity{} == activity) {
		log.Printf("\tactivity id '%s' found in file", activityId)
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

// @Summary Get today's activities
// @Description Get all activities recorded today in CSV format
// @Tags activity
// @Produce json
// @Success 200 {array} Activity "List of today's activities"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "No activities found for today"
// @Router /activity/csv/today [get]
func (h *ActivityManager) getTodayCsv(w http.ResponseWriter) {

	log.Println("activity manager - request for today's CSV received")

	// Generate today's filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	log.Printf("\tlooking for file: %s\n", filename)
	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Println("\tfile not found, likely no data saved today")
		http.Error(w, "No activity data for today", http.StatusNotFound)
		return
	}

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Println("\terror unable to open file")
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
		log.Printf("\terror responding with data: %v", err)
		log.Printf("Error sending CSV file: %v", err)
	}

	log.Println("\ttoday's CSV returned to caller")
}

// @Summary Get activities by date
// @Description Get all activities recorded on a specific date in CSV format
// @Tags activity
// @Produce json
// @Param date path string true "Date in YYYYMMDD format"
// @Success 200 {array} Activity "List of activities for the specified date"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "No activities found for the specified date"
// @Router /activity/csv/{date} [get]
func (h *ActivityManager) getCsvByDate(w http.ResponseWriter, r *http.Request) {

	log.Println("activity manager - request for dated CSV received")

	// Extract date from URL using regex patter
	matches := activityCsvByDate.FindStringSubmatch(r.URL.String())
	if len(matches) < 1 {
		log.Printf("\tinvalid date received in URL")
		http.Error(w, "Invalid date in URL", http.StatusBadRequest)
		return
	}
	fileDate := matches[1]
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", fileDate)

	log.Printf("\tdate for CSV request is '%s'\n", fileDate)

	file, err := getCsvFile(filename)
	if err != nil {
		log.Printf("\tunable to get CSV file: %s", err.Error())
		http.Error(w, fmt.Sprintf("error opening CSV file '%s'  %s", filename, err.Error()), http.StatusInternalServerError)
		return
	}
	defer file.Close() // Ensure the file is closed after we're done with it

	// Set response headers for CSV file download
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	// Copy the file contents to the response
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Error sending CSV file: "+err.Error(), http.StatusInternalServerError)
	}

	log.Printf("\tdata for CSV for '%s' returned to caller", fileDate)

}

// @Summary Get activity by date and ID
// @Description Get a specific activity by its date and ID
// @Tags activity
// @Produce json
// @Param date path string true "Date in YYYYMMDD format"
// @Param id path string true "Activity ID"
// @Success 200 {object} Activity "Activity details"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "Activity not found"
// @Router /activity/{date}/{id} [get]
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

// @Summary Get activity by ID
// @Description Get a specific activity by its ID (from today's activities)
// @Tags activity
// @Produce json
// @Param id path string true "Activity ID"
// @Success 200 {object} Activity "Activity details"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "Activity not found"
// @Router /activity/{id} [get]
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

	// TODO - I've changed this so that if a file for the date isn't found or an id isn't found
	// in the file that was opened I'm just returning an empty Activity. Eventually should rethink
	// this to actually tell the caller what's up.

	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Printf("No activity data for activity id: %s\n", activityId)
		return Activity{}, nil
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

	log.Printf("activity not found: %s\n", activityId)
	return Activity{}, nil
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

func getCsvFile(fileName string) (*os.File, error) {
	// Check if the file exists
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil, fmt.Errorf("no activity data file '%s' found", fileName)
	}

	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening csv file: %v", err)
	}

	return file, nil
}

// @Summary Post activity to Jira/Tempo
// @Description Post an activity to Jira/Tempo by its date and ID
// @Tags activity
// @Produce json
// @Param date path string true "Date in YYYYMMDD format"
// @Param id path string true "Activity ID"
// @Success 200 {object} string "Successfully posted to Jira/Tempo"
// @Failure 400 {object} string "Bad request"
// @Failure 404 {object} string "Activity not found"
// @Router /activity/tempo/{date}/{id} [post]
func (h *ActivityManager) activityToTempoById(w http.ResponseWriter, r *http.Request) {
	// Look up id in today's file, create payload to send to Jira endpoint
	// NOTE that as of now, the endpoint is completely faked out
	// Example paylaod:
	/*
		{
		    "issueKey": "FEDS-148",
		    "timeSpentSeconds": 3600,
		    "startDate": "2025-05-13",
		    "description": "Working on task"
		  }
	*/
	// So need to convert the stored duration to seconds, and the start date to just YYYY-MM-DD

	// Extract activity ID from URL using the regex pattern
	matches := activityToTempo.FindStringSubmatch(r.URL.String())
	if len(matches) < 2 {
		http.Error(w, "Invalid activity ID in URL", http.StatusBadRequest)
		return
	}
	activityId := matches[2]
	fileDate := matches[1]

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

	// Check if the activity has already been posted to Jira/Tempo
	if activity.PostedToJiraTempo {
		response := map[string]string{"message": "Activity has already been posted to Jira/Tempo"}
		responseJSON, _ := json.Marshal(response)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(responseJSON)
		return
	}

	durationInSeconds, err := getDurationInSeconds(activity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jiraTempoPayload := JiraTempoPayload{
		IssueKey:         activity.Jira,
		TimeSpentSeconds: durationInSeconds,
		Description:      activity.InputDescription,
		StartDate:        activity.CreatedAt.Format("20060102"),
	}

	// Post to Jira endpoint
	requestData, err := json.Marshal(jiraTempoPayload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error marshalling request: %w", err), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest("POST", jiraTempoEndpoint, bytes.NewBuffer(requestData))
	if err != nil {
		http.Error(w, fmt.Sprintf("error creating request: %w", err), http.StatusBadRequest)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("error sending request to Jira/Tempo: %w", err), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Jira/Tempo API returned error: %s - %s", resp.Status, string(responseBody)), http.StatusBadRequest)
		return
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading response body: %w", err), http.StatusBadRequest)
		return
	}

	// Update the activity to mark it as posted to Jira/Tempo
	activity.PostedToJiraTempo = true
	err = updateActivityInCSV(activity, filename)
	if err != nil {
		// Even if we fail to update the file, we still successfully posted to Jira/Tempo
		// So we'll log the error but still return success to the client
		log.Printf("Error updating activity in file: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
	return

}
