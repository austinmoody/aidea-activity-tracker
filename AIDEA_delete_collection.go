package main

import (
	"context"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
)

func deleteCollection() {
	cfg := weaviate.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Delete the ActivityRules collection
	err = client.Schema().ClassDeleter().WithClassName("ActivityRules").Do(context.Background())
	if err != nil {
		fmt.Printf("Error deleting collection: %v\n", err)
		return
	}

	fmt.Println("ActivityRules collection successfully deleted")
}
