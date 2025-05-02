package main

import (
	"context"
	"fmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

func categorizeActivity(activity Activity) Activity {
	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		fmt.Println(err)
	}

	ctx := context.Background()

	// TODO read systemPrompt from file & make it better
	systemPrompt := "Find the Activity Description that most matches the supplied string."

	gs := graphql.NewGenerativeSearch().GroupedResult(systemPrompt)

	response, err := client.GraphQL().Get().
		WithClassName("ActivityRules").
		WithFields(
			graphql.Field{Name: "project"},
			graphql.Field{Name: "task"},
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
				WithConcepts([]string{activity.InputDescription}),
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

		activity.WeaviateId = weaviateId
		activity.Project = rule["project"].(string)
		activity.Task = rule["task"].(string)
		activity.RuleDescription = rule["description"].(string)
		activity.Jira = rule["jira"].(string)
		activity.CategorizationDistance = distance
		activity.CategorizationGrade = getCategorizationGrade(distance)
	} else {
		fmt.Printf("No activity category found in response")
	}

	return activity
}

func getCategorizationGrade(distance float64) string {
	switch {
	case distance >= 0.0 && distance < 0.2:
		return "A"
	case distance >= 0.2 && distance < 0.4:
		return "B"
	case distance >= 0.4 && distance < 0.7:
		return "C"
	case distance >= 0.7 && distance < 1.0:
		return "D"
	default:
		return "F"
	}
}
