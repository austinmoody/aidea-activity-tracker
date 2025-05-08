package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"
)

func saveActivityCsv(activity Activity) error {

	// TODO - output directory as configuration
	// TODO - save in some kind of data store

	// Generate filename based on current date
	currentDate := time.Now().Format("20060102") // Format for YYYYMMDD
	filename := fmt.Sprintf("aidea_activity_tracking_%s.csv", currentDate)

	// Check if the file exists to determine if we need to write headers
	fileExists := false
	if _, err := os.Stat(filename); err == nil {
		fileExists = true
	}

	// Open file append mode or create if it doesn't exist
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("couldn't open csv file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if !fileExists {
		// Write headers if this file didn't exist already
		if err := writer.Write(getHeaders(activity)); err != nil {
			return fmt.Errorf("error writing headers: %v", err)
		}
	}

	// Write data in Activity
	myValues := getActivitySlice(activity)
	log.Printf("writing %d records to csv", len(myValues))

	if err := writer.Write(myValues); err != nil {
		return fmt.Errorf("error writing records to csv: %v", err)
	}

	return nil
}

func getRuleHeaders(rule Rule) []string {
	ruleType := reflect.TypeOf(rule)

	headers := make([]string, ruleType.NumField())
	for i := 0; i < ruleType.NumField(); i++ {
		headers[i] = ruleType.Field(i).Name
	}
	return headers
}

// Take an Rule object and convert it to a []string to save as CSV
// Looping the fields in the Rule struct to keep order with the
// headers we already created.
func getRuleSlice(rule Rule) []string {
	ruleType := reflect.TypeOf(rule)
	ruleValue := reflect.ValueOf(rule)

	ruleValues := make([]string, ruleValue.NumField())
	for i := 0; i < ruleType.NumField(); i++ {
		field := ruleValue.Field(i)

		// Convert each field to string appropriately
		switch field.Kind() {
		case reflect.String:
			ruleValues[i] = field.String()
		case reflect.Float64:
			ruleValues[i] = fmt.Sprintf("%f", field.Float())
		case reflect.Struct:
			// Check if this is a time.Time field and format it nicely
			if t, ok := field.Interface().(time.Time); ok {
				ruleValues[i] = t.Format("2006-01-02 15:04:05")
			} else {
				ruleValues[i] = fmt.Sprintf("%v", field.Interface())
			}
		case reflect.Bool:
			ruleValues[i] = fmt.Sprintf("%t", field.Bool())
		// Add other types as needed
		default:
			ruleValues[i] = fmt.Sprintf("%v", field.Interface())
		}
	}
	return ruleValues
}

func getHeaders(activity Activity) []string {
	activityType := reflect.TypeOf(activity)

	headers := make([]string, activityType.NumField())
	for i := 0; i < activityType.NumField(); i++ {
		headers[i] = activityType.Field(i).Name
	}
	return headers
}

// Take an Activity object and convert it to a []string to save as CSV
// Looping the fields in the Activity struct to keep order with the
// headers we already created.
func getActivitySlice(activity Activity) []string {
	activityType := reflect.TypeOf(activity)
	activityValue := reflect.ValueOf(activity)

	activityValues := make([]string, activityValue.NumField())
	for i := 0; i < activityType.NumField(); i++ {
		field := activityValue.Field(i)

		// Convert each field to string appropriately
		switch field.Kind() {
		case reflect.String:
			activityValues[i] = field.String()
		case reflect.Float64:
			activityValues[i] = fmt.Sprintf("%f", field.Float())
		case reflect.Struct:
			// Check if this is a time.Time field and format it nicely
			if t, ok := field.Interface().(time.Time); ok {
				activityValues[i] = t.Format("2006-01-02 15:04:05")
			} else {
				activityValues[i] = fmt.Sprintf("%v", field.Interface())
			}
		case reflect.Bool:
			activityValues[i] = fmt.Sprintf("%t", field.Bool())
		// Add other types as needed
		default:
			activityValues[i] = fmt.Sprintf("%v", field.Interface())
		}
	}
	return activityValues
}
