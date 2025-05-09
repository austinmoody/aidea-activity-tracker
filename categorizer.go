package main

import (
	"context"
	"fmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"slices"
)

func categorizeActivity(activity Activity) Activity {
	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		fmt.Println(err)
	}

	ctx := context.Background()

	// TODO read systemPrompt from file & make it better
	systemPrompt := `You are a work time activity categorizer. 
Find the Activity Description that most matches the supplied string
Input may include descriptions of how much time was spent on the task, do not include this information
in your categorization.
Some concepts which may be helpful:
IZG means IZ Gateway or Immunization Gateway
Transformation Service is the same as Xform Service
Transformation Console is the same as Xform Console
IZG CC is the IZG Configuration Console or IZ Gateway Configuration Console
`

	gs := graphql.NewGenerativeSearch().GroupedResult(systemPrompt)

	response, err := client.GraphQL().Get().
		WithClassName(weaviateClass).
		WithFields(
			graphql.Field{Name: "project"},
			graphql.Field{Name: "task"},
			graphql.Field{Name: "jira"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "distance"},         // Default weaviate uses cosine.  0 = identical vector / 2 = opposing vector
				{Name: "id"},               // Internal Weaviate identifier
				{Name: "creationTimeUnix"}, // Internal Weaviate creation date/time
			}},
		).
		WithGenerativeSearch(gs).
		WithNearText(
			client.GraphQL().NearTextArgBuilder().
				WithConcepts([]string{activity.InputDescription}),
		).
		WithLimit(10).
		Do(ctx)

	if err != nil {
		panic(err)
	}

	// Extract data from response
	data := response.Data["Get"].(map[string]interface{})
	activityRules := data[weaviateClass].([]interface{})

	if len(activityRules) > 0 {
		// Get the first result
		rule := activityRules[0].(map[string]interface{})

		additional := rule["_additional"].(map[string]interface{})
		distance := additional["distance"].(float64)
		weaviateId := additional["id"].(string)

		// Get the grade so we can determine if we want to save the result
		activity.CategorizationGrade = getCategorizationGrade(distance)

		if slices.Contains(autoGrades, activity.CategorizationGrade) {
			// We are only going to save this information if the categorization
			// matches configured grade(s)
			activity.Project = rule["project"].(string)
			activity.Task = rule["task"].(string)
			activity.Jira = rule["jira"].(string)
			activity.Categorized = true
		}

		// Save these no matter what so that user can see what the "closest" match was
		activity.WeaviateId = weaviateId
		activity.RuleDescription = rule["description"].(string)
		activity.CategorizationDistance = distance
		activity.CategorizationGrade = getCategorizationGrade(distance)
	} else {
		activity.Categorized = false
		activity.CategorizationGrade = "N/A"
		activity.RuleDescription = "N/A"
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
