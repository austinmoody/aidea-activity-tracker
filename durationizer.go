package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OllamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	System      string  `json:"system"`
	Stream      bool    `json:"stream"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type OllamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func getDuration(activity Activity) (string, error) {

	systemPrompt := `Extract any time information from the input and convert it to hours and minutes format
  - IF NO SPECIFIC TIME IS MENTIONED IN THE INPUT, USE EXACTLY "15m" AS THE DEFAULT
  - For specific time mentions, convert to the format "Xh Ym" where X is hours and Y is minutes
  - When the total minutes are 60 or more, convert to hours and remaining minutes:
    - Example: 75 minutes = 1 hour and 15 minutes = "1h 15m"
    - Example: 90 minutes = 1 hour and 30 minutes = "1h 30m"
    - Example: 120 minutes = 2 hours = "2h"
    - Example: 150 minutes = 2 hours and 30 minutes = "2h 30m"
  - If the time is less than one hour, use only minutes:
    - Example: 30 minutes = "30m"
    - Example: 45 minutes = "45m"
  - If the time is an exact number of hours, omit the minutes:
    - Example: 2 hours = "2h"
    - Example: 1 hour = "1h"
  - If no specific time is mentioned, respond with a default of "15m"
  - For time conversion:
    - 60 minutes = 1 hour
    - To convert minutes to hours and minutes: divide by 60 to get hours, use remainder for minutes`

	ollamaRequest := OllamaRequest{
		Model:       ollamaGenModel,
		Prompt:      activity.InputDescription,
		System:      systemPrompt,
		Stream:      false,
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	requestData, err := json.Marshal(ollamaRequest)
	if err != nil {
		return "", fmt.Errorf("error marshalling request: %w", err)
	}

	req, err := http.NewRequest("POST", ollamaGenEndpoint, bytes.NewBuffer(requestData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API returned error: %s - %s", resp.Status, string(responseBody))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var ollamaResponse OllamaResponse
	err = json.Unmarshal(responseBody, &ollamaResponse)
	if err != nil {
		return "", fmt.Errorf("error processing Ollama response: %w", err)
	}

	return ollamaResponse.Response, nil

}
