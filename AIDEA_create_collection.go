package main

import (
	"context"
	"fmt"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
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

	// Define the collection
	classObj := &models.Class{
		Class:      "ActivityRules",
		Vectorizer: "text2vec-ollama",
		ModuleConfig: map[string]interface{}{
			"text2vec-ollama": map[string]interface{}{ // Configure the Ollama embedding integration
				"apiEndpoint": "http://host.docker.internal:11434", // Allow Weaviate from within a Docker container to contact your Ollama instance
				"model":       "all-minilm",                        // The model to use
			},
			"generative-ollama": map[string]interface{}{ // Configure the Ollama generative integration
				"apiEndpoint": "http://host.docker.internal:11434", // Allow Weaviate from within a Docker container to contact your Ollama instance
				"model":       "gemma3",                            // The model to use
			},
		},
		Properties: []*models.Property{
			{
				Name:     "rule_id",
				DataType: []string{"int"},
			},
			{
				Name:     "category",
				DataType: []string{"text"},
			},
			{
				Name:     "jira",
				DataType: []string{"text"},
			},
			{
				Name:     "description",
				DataType: []string{"text"},
			},
		},
	}

	// add the collection
	err = client.Schema().ClassCreator().WithClass(classObj).Do(context.Background())
	if err != nil {
		panic(err)
	}
}
