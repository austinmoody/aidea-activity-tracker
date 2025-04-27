package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

type ActivityRule struct {
	RuleId      int    `json:"rule_id"`
	Category    string `json:"category"`
	Jira        string `json:"jira"`
	Description string `json:"description"`
}
type ActivityRules struct {
	Activities []ActivityRule `json:"activities"`
}

func main() {
	cfg := weaviate.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		fmt.Println(err)
	}

	// Read in SITE data and process
	data, err := os.ReadFile("activity_data_SITE_withid.json")
	if err != nil {
		panic(err)
	}

	var siteDescriptions ActivityRules
	if err := json.Unmarshal(data, &siteDescriptions); err != nil {
		fmt.Printf("Error parsing config: %v\n", err)
		os.Exit(1)
	}

	// convert items into a slice of models.Object
	objects := make([]*models.Object, len(siteDescriptions.Activities))
	for i, activity := range siteDescriptions.Activities {
		objects[i] = &models.Object{
			Class: "ActivityRules",
			Properties: map[string]any{
				"rule_id":     activity.RuleId,
				"category":    activity.Category,
				"jira":        activity.Jira,
				"description": activity.Description,
			},
		}
	}

	// batch write items
	batchRes, err := client.Batch().ObjectsBatcher().WithObjects(objects...).Do(context.Background())
	if err != nil {
		panic(err)
	}
	for _, res := range batchRes {
		if res.Result.Errors != nil {
			panic(res.Result.Errors.Error)
		}
	}
}
