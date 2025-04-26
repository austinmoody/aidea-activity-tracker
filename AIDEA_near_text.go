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

	//TODO - check the limit... the description below SHOULD NOT return anything from the SITE data
	response, err := client.GraphQL().Get().
		WithClassName("ActivityDescriptions").
		WithFields(
			graphql.Field{Name: "category"},
			graphql.Field{Name: "jira"},
			graphql.Field{Name: "description"},
		).
		WithNearText(client.GraphQL().NearTextArgBuilder().
			WithConcepts([]string{"Coding on Xform Service organizations api"})).
		WithLimit(2).
		Do(ctx)

	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", response)
}
