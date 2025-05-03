package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func importRules() {

	log.Printf("loading rule files from '%s'\n", rulesDirectory)

	// Loop files in rules directory
	ruleFiles, err := os.ReadDir(rulesDirectory)
	if err != nil {
		log.Fatal(fmt.Sprintf("issue reading rules directory '%s'", rulesDirectory), err)
	}
	for _, ruleFile := range ruleFiles {
		if !ruleFile.IsDir() {
			// We have a file, pass to loader
			loadRuleFile(ruleFile)
		}
	}
}

func loadRuleFile(ruleFile os.DirEntry) {

	log.Printf("\tloading rule file '%s'", ruleFile.Name())

	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		fmt.Println(err)
	}

	data, err := os.ReadFile(fmt.Sprintf("%s/%s", rulesDirectory, ruleFile.Name()))
	if err != nil {
		fmt.Printf("Error reading rule file '%s': %v\n", ruleFile.Name(), err)
		os.Exit(1)
	}

	var rules ActivityRules
	if err := json.Unmarshal(data, &rules); err != nil {
		fmt.Printf("Error parsing rule file '%s': %v\n", ruleFile.Name(), err)
		os.Exit(1)
	}

	log.Printf("\t\trule file contained %d rules", len(rules.Activities))

	// convert items into a slice of models.Object
	objects := make([]*models.Object, len(rules.Activities))
	for i, activity := range rules.Activities {
		objects[i] = &models.Object{
			Class: weaviateClass,
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

	log.Print("\t\trule file has been loaded")

}
