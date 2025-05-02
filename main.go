package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	trackerPort     string
	weaviateConfig  weaviate.Config
	weaviateClass   string
	embedModel      string
	generativeModel string
	ollamaEndpoint  string
	rulesDirectory  string
)

type Activity struct {
	ActivityId             string    `json:"activity_id"`
	WeaviateId             string    `json:"weaviate_id"`
	Project                string    `json:"project"`
	Task                   string    `json:"task"`
	Jira                   string    `json:"jira"`
	InputDescription       string    `json:"input_description"`
	RuleDescription        string    `json:"rule_description"`
	CategorizationDistance float64   `json:"categorization_distance"`
	CategorizationGrade    string    `json:"categorization_grade"`
	CreatedAt              time.Time `json:"created_at"`
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	trackerPort = os.Getenv("TRACKER_PORT")

	weaviateConfig = weaviate.Config{
		Host:   fmt.Sprintf("%s:%s", os.Getenv("WEAVIATE_HOST"), os.Getenv("WEAVIATE_PORT")),
		Scheme: os.Getenv("WEAVIATE_PROTOCOL"),
	}

	weaviateClass = os.Getenv("WEAVIATE_CLASS")
	embedModel = os.Getenv("WEAVIATE_OLLAMA_EMBED_MODEL")
	generativeModel = os.Getenv("WEAVIATE_OLLAMA_GEN_MODEL")
	ollamaEndpoint = os.Getenv("WEAVIATE_OLLAMA_ENDPOINT")
	rulesDirectory = os.Getenv("RULES_DIRECTORY")
}

func main() {

	log.Printf("startup - AIdea Activity Tracker")

	// check the weaviate collection
	collectionCheck()

	// Import rule files
	//importRules()

	mux := http.NewServeMux()
	mux.Handle("/api/v1/activity", &ActivityManager{})
	mux.Handle("/api/v1/rule", &RuleManager{})

	fmt.Printf("starting server on port '%s'", trackerPort)
	err := http.ListenAndServe(fmt.Sprintf(":%s", trackerPort), mux)
	if err != nil {
		log.Fatal("issue starting server: ", err)
	}

	log.Printf("finished starting server on port '%s'", trackerPort)
}
