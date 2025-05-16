package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

// TODO - generic Ollama function to pass in system prompt, user input and get response

func getDuration(activity Activity) (string, error) {

	systemPrompt := `You are a time duration extractor. Your ONLY job is to output a time duration in the format below.

CRITICAL INSTRUCTIONS:
1. NEVER include any explanations, questions, or additional text in your response
2. ONLY output the final time duration and nothing else
3. DO NOT respond conversationally under any circumstances
4. Your ENTIRE response must be JUST the duration string

Format rules:
- ALWAYS OUTPUT EXACTLY "15m" if no specific time is mentioned in the input
- For specific time mentions, convert to the format "Xh Ym" where X is hours and Y is minutes
- For hours + minutes format:
  - Example: 75 minutes = "1h 15m"
  - Example: 90 minutes = "1h 30m" 
  - Example: 120 minutes = "2h"
  - Example: 150 minutes = "2h 30m"
- For minutes only (less than one hour):
  - Example: 30 minutes = "30m"
  - Example: 45 minutes = "45m"
- For exact hours:
  - Example: 2 hours = "2h"
  - Example: 1 hour = "1h"

Examples:
Input: "Working on project for 30 minutes"
Output: 30m

Input: "Spent 2 hours on bug fixes"
Output: 2h

Input: "Meeting lasted 1 hour and 15 minutes"
Output: 1h 15m

Input: "Working on AIdea"
Output: 15m

Input: "Coding the new feature"
Output: 15m`

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

func getDurationInSeconds(activity Activity) (int, error) {

	systemPrompt := `You are a time duration extractor. Your ONLY job is to output a time duration in seconds.
CRITICAL INSTRUCTIONS:
1. NEVER include any explanations, questions, or additional text in your response
2. ONLY output the final time duration in seconds and nothing else
3. DO NOT respond conversationally under any circumstances
4. Your ENTIRE response must be JUST the duration integer in seconds

The time format you will receive is "Xh Ym" where X is hours and Y is minutes.  

If the duration only contains minutes you would only receive Ym

If the duration only contains hours you would only receive Xh

You will need to convert this to seconds

So if you received 30m you would return 1800

If you received 1h you would return 3600

If you received 2h 15m you would return 8100
`
	ollamaRequest := OllamaRequest{
		Model:       ollamaGenModel,
		Prompt:      activity.Duration,
		System:      systemPrompt,
		Stream:      false,
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	requestData, err := json.Marshal(ollamaRequest)
	if err != nil {
		return -1, fmt.Errorf("error marshalling request: %w", err)
	}

	req, err := http.NewRequest("POST", ollamaGenEndpoint, bytes.NewBuffer(requestData))
	if err != nil {
		return -1, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, fmt.Errorf("error sending request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return -1, fmt.Errorf("Ollama API returned error: %s - %s", resp.Status, string(responseBody))
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Errorf("error reading response body: %w", err)
	}

	var ollamaResponse OllamaResponse
	err = json.Unmarshal(responseBody, &ollamaResponse)
	if err != nil {
		return -1, fmt.Errorf("error processing Ollama response: %w", err)
	}

	durationAsString := strings.TrimSuffix(ollamaResponse.Response, "\n")

	return strconv.Atoi(durationAsString)
}
