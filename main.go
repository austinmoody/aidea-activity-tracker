package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	trackerPort             string
	weaviateConfig          weaviate.Config
	weaviateClass           string
	weaviateEmbedModel      string
	weaviateGenerativeModel string
	weaviateOllamaEndpoint  string
	rulesDirectory          string
	// There are two ollama endpoints only because Weaviate is
	// running in Docker and so needs host.docker.internal to talk
	// to my locally running Ollama. But I can't get a duration pulled
	// via Weaviate so I'm making a 2nd call to Ollama (locally) to
	// get that until something else can happen
	ollamaGenEndpoint string
	ollamaGenModel    string
	autoGrades        []string
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
	Duration               string    `json:"duration"`
	Categorized            bool      `json:"categorized"`
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
	weaviateEmbedModel = os.Getenv("WEAVIATE_OLLAMA_EMBED_MODEL")
	weaviateGenerativeModel = os.Getenv("WEAVIATE_OLLAMA_GEN_MODEL")
	weaviateOllamaEndpoint = os.Getenv("WEAVIATE_OLLAMA_ENDPOINT")
	rulesDirectory = os.Getenv("RULES_DIRECTORY")

	ollamaGenEndpoint = os.Getenv("OLLAMA_GEN_ENDPOINT")
	ollamaGenModel = os.Getenv("OLLAMA_GEN_MODEL")

	// Whatever is set here will be "categorized".  So, currently when categorizer
	// runs it looks as the distance and determines a A,B,C,D,F "grade". Whatever
	// is set here gets automatically categorized. So if you have this set to A,B
	// and the match is determined to be C the categorization won't be saved
	autoGrades = strings.Split(os.Getenv("AUTO_CATEGORIZE_GRADES"), ",")

}

func main() {

	log.Printf("startup - AIdea Activity Tracker")

	// check the weaviate collection
	collectionCheck()

	mux := http.NewServeMux()
	mux.Handle("/api/v1/activity/", &ActivityManager{})
	mux.Handle("/api/v1/activity", &ActivityManager{})
	mux.Handle("/api/v1/rule", &RuleManager{})
	mux.Handle("/api/v1/rule/", &RuleManager{})
	mux.Handle("/api/v1/project", &ProjectManager{})
	mux.Handle("/api/v1/project/", &ProjectManager{})

	log.Printf("startup - server on port '%s'", trackerPort)
	err := http.ListenAndServe(fmt.Sprintf(":%s", trackerPort), mux)
	if err != nil {
		log.Fatal("issue starting server: ", err)
	}
}
