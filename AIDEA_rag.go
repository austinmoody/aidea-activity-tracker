package main

import (
	"context"
	"fmt"
	"os"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

type Activity struct {
	Id               string `json:"id"`
	Category         string `json:"category"`
	Jira             string `json:"jira"`
	InputDescription string `json:"input_description"`
	RuleDescription  string `json:"rule_description"`
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

	ctx := context.Background()

	userWorkDescription := "Austin Moody is my name"

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
			graphql.Field{Name: "category"},
			graphql.Field{Name: "jira"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "certainty"}, // This gives you a confidence score between 0-1
				{Name: "distance"},  // This gives you vector distance (lower is better)
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
	fmt.Printf("%v", response)
}
