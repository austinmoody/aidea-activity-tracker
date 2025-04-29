package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
)

var (
	trackerPort string
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	trackerPort = os.Getenv("TRACKER_PORT")
}

func main() {

	log.Printf("startup - AIdea Activity Tracker")

	// check the weaviate collection
	collectionCheck()

	mux := http.NewServeMux()

	fmt.Printf("starting server on port '%s'", trackerPort)
	err := http.ListenAndServe(fmt.Sprintf(":%s", trackerPort), mux)
	if err != nil {
		log.Fatal("issue starting server: ", err)
	}

	log.Printf("finished starting server on port '%s'", trackerPort)
}
