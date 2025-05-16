package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
)

// TODO - I know this is awful... storing the projects in environment
// If I do anything with this beyond the AIDEA thing we'd store this in
// proper way.

// TODO - Sometimes think that I should make it so that when a user is
// saving an activity they should also specify the project at least but
// that kind of complicates that flow. And I'm falling into database
// structure

// TODO - I'd added this so that a UI (or Apple Shortcuts) could get a list
// of project -> task -> jira combinations to present to the user in order
// for them to add a Rule.  That is currently NOT in use, running out of time

type Project struct {
	ProjectName string `json:"project"`
	Task        string `json:"task"`
	Jira        string `json:"jira"`
}

type ProjectManager struct{}

var (
	projects []Project
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	for i := 1; ; i++ {
		projectKey := fmt.Sprintf("PROJECT_%d_NAME", i)
		projectName := os.Getenv(projectKey)

		// Break if no more projects
		if projectName == "" {
			break
		}

		taskKey := fmt.Sprintf("PROJECT_%d_TASK", i)
		jiraKey := fmt.Sprintf("PROJECT_%d_JIRA", i)

		project := Project{
			ProjectName: projectName,
			Task:        os.Getenv(taskKey),
			Jira:        os.Getenv(jiraKey),
		}

		projects = append(projects, project)
	}
}

func (h *ProjectManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == "GET":
		h.getProjects(w)
	default:
		http.Error(w, "invalid request", http.StatusBadRequest)
	}
}

func (h *ProjectManager) getProjects(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(projects)
	return
}
