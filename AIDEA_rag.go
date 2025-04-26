package main

import (
	"context"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

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

	generatePrompt := "Can you tell me which activity matches closest to the description given?"

	gs := graphql.NewGenerativeSearch().GroupedResult(generatePrompt)

	response, err := client.GraphQL().Get().
		WithClassName("ActivityDescriptions").
		WithFields(
			graphql.Field{Name: "category"},
			graphql.Field{Name: "jira"},
			graphql.Field{Name: "description"},
		).
		WithGenerativeSearch(gs).
		WithNearText(client.GraphQL().NearTextArgBuilder().
			WithConcepts([]string{"Coding on Xform Service organizations api"})).
		WithLimit(1).
		Do(ctx)

	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", response)
}
