package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

func ragTest() {
	cfg := weaviate.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		fmt.Println(err)
	}

	ctx := context.Background()

	userWorkDescription := "IZG Xform Service and Xform Console - cut releases"

	systemPromptFile, err := os.ReadFile("system_prompt.txt")
	if err != nil {
		fmt.Printf("Error reading system prompt file: %v\n", err)
		os.Exit(1)
	}

	systemPrompt := fmt.Sprintf(string(systemPromptFile), userWorkDescription)

	systemPrompt = "Find the Activity Description that most matches the supplied string."

	gs := graphql.NewGenerativeSearch().GroupedResult(systemPrompt)

	response, err := client.GraphQL().Get().
		WithClassName("ActivityRules").
		WithFields(
			graphql.Field{Name: "rule_id"},
			graphql.Field{Name: "category"},
			graphql.Field{Name: "jira"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "distance"}, // Default weaviate uses cosine.  0 = identical vector / 2 = opposing vector
				{Name: "id"},       // Internal Weaviate identifier
			}},
		).
		WithGenerativeSearch(gs).
		WithNearText(
			client.GraphQL().NearTextArgBuilder().
				WithConcepts([]string{userWorkDescription}),
		).
		WithLimit(1).
		Do(ctx)

	if err != nil {
		panic(err)
	}

	// Extract data from response
	data := response.Data["Get"].(map[string]interface{})
	activityRules := data["ActivityRules"].([]interface{})

	if len(activityRules) > 0 {
		// Get the first result
		rule := activityRules[0].(map[string]interface{})

		additional := rule["_additional"].(map[string]interface{})
		distance := additional["distance"].(float64)
		weaviateId := additional["id"].(string)

		// Debug the response structure
		fmt.Printf("Rule data types: %T %T %T %T\n",
			rule["rule_id"], rule["description"], rule["category"], rule["jira"])

		// Create new Activity from the result
		activity := Activity{
			ActivityId:             uuid.New().String(),
			WeaviateId:             weaviateId,
			RuleId:                 fmt.Sprintf("%v", rule["rule_id"]),
			RuleDescription:        fmt.Sprintf("%v", rule["description"]),
			Category:               fmt.Sprintf("%v", rule["category"]),
			Jira:                   fmt.Sprintf("%v", rule["jira"]),
			InputDescription:       userWorkDescription,
			CategorizationDistance: distance,
		}

		fmt.Printf("Activity created: %+v\n", activity)
	} else {
		fmt.Println("No matching activity rules found")
	}
}
