package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/fault"
	"log"
	"os"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"
)

func collectionCheck() {

	log.Printf("checking for collection '%s'\n", weaviateClass)

	client, err := weaviate.NewClient(weaviateConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Define the collection
	classObj := &models.Class{
		Class:      weaviateClass,
		Vectorizer: "text2vec-ollama",
		ModuleConfig: map[string]interface{}{
			"text2vec-ollama": map[string]interface{}{
				"apiEndpoint": weaviateOllamaEndpoint,
				"model":       weaviateEmbedModel, // Embedding model to use
			},
			"generative-ollama": map[string]interface{}{
				"apiEndpoint": weaviateOllamaEndpoint,
				"model":       weaviateGenerativeModel, // Generative model to use
			},
		},
		// TODO - build this from Rule struct in rule_manager.go
		Properties: []*models.Property{
			{
				Name:     "project",
				DataType: []string{"text"},
			},
			{
				Name:     "task",
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

	// Check to see if the collection exists already
	_, err = client.Schema().ClassGetter().WithClassName(classObj.Class).Do(context.Background())
	weaviateClassExists := true
	if err != nil {
		wce := &fault.WeaviateClientError{}
		if errors.As(err, &wce) && wce.StatusCode == 404 {
			weaviateClassExists = false
		} else {
			fmt.Printf("error getting existing collection: %v\n", err)
			os.Exit(1)
		}
	}

	if weaviateClassExists == false { // add the collection
		log.Printf("collection '%s' not found, will create", classObj.Class)
		err = client.Schema().ClassCreator().WithClass(classObj).Do(context.Background())
		if err != nil {
			fmt.Printf("error adding collection: '%v'\n", err)
			os.Exit(1)
		}
	} else {
		log.Printf("collection '%s' already exists", classObj.Class)
	}

	// TODO - may want way to update collection if class name exists but parameters are different

}
