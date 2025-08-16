package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebSearch struct {
	enabled bool
}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
}

func NewWebSearch() *WebSearch {
	return &WebSearch{
		enabled: true, // Can be controlled via config
	}
}

// SearchForSolution searches for programming solutions using DuckDuckGo Instant Answer API
func (ws *WebSearch) SearchForSolution(query string) (*SearchResponse, error) {
	if !ws.enabled {
		return nil, fmt.Errorf("web search is disabled")
	}

	// Enhance query for programming-specific results
	enhancedQuery := fmt.Sprintf("golang %s site:stackoverflow.com OR site:github.com OR site:pkg.go.dev", query)
	
	return ws.performSearch(enhancedQuery)
}

// SearchForError searches specifically for error solutions
func (ws *WebSearch) SearchForError(errorMessage string) (*SearchResponse, error) {
	if !ws.enabled {
		return nil, fmt.Errorf("web search is disabled")
	}

	// Clean up error message for search
	cleanError := ws.cleanErrorMessage(errorMessage)
	query := fmt.Sprintf("golang \"%s\" solution fix", cleanError)
	
	return ws.performSearch(query)
}

func (ws *WebSearch) performSearch(query string) (*SearchResponse, error) {
	// Use DuckDuckGo Instant Answer API (no API key required)
	baseURL := "https://api.duckduckgo.com/"
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	params.Add("no_html", "1")
	params.Add("skip_disambig", "1")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse DuckDuckGo response
	var ddgResponse map[string]interface{}
	if err := json.Unmarshal(body, &ddgResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our format
	searchResp := &SearchResponse{
		Query:   query,
		Results: []SearchResult{},
	}

	// Extract instant answer if available
	if abstract, ok := ddgResponse["Abstract"].(string); ok && abstract != "" {
		if abstractURL, ok := ddgResponse["AbstractURL"].(string); ok {
			searchResp.Results = append(searchResp.Results, SearchResult{
				Title:   "Solution Summary",
				URL:     abstractURL,
				Snippet: abstract,
			})
		}
	}

	// Extract related topics
	if relatedTopics, ok := ddgResponse["RelatedTopics"].([]interface{}); ok {
		for i, topic := range relatedTopics {
			if i >= 3 { // Limit to top 3 results
				break
			}
			if topicMap, ok := topic.(map[string]interface{}); ok {
				if text, ok := topicMap["Text"].(string); ok {
					if firstURL, ok := topicMap["FirstURL"].(string); ok {
						searchResp.Results = append(searchResp.Results, SearchResult{
							Title:   "Related Solution",
							URL:     firstURL,
							Snippet: text,
						})
					}
				}
			}
		}
	}

	return searchResp, nil
}

func (ws *WebSearch) cleanErrorMessage(errorMsg string) string {
	// Extract the core error message, removing file paths and line numbers
	lines := strings.Split(errorMsg, "\n")
	if len(lines) == 0 {
		return errorMsg
	}

	// Take first non-empty line and clean it
	errorLine := strings.TrimSpace(lines[0])
	
	// Remove common prefixes
	prefixes := []string{
		"Error: ",
		"error: ",
		"go: ",
		"build failed: ",
	}
	
	for _, prefix := range prefixes {
		if strings.HasPrefix(errorLine, prefix) {
			errorLine = strings.TrimPrefix(errorLine, prefix)
			break
		}
	}

	// Remove file paths (anything that looks like a path)
	parts := strings.Fields(errorLine)
	var cleanParts []string
	for _, part := range parts {
		// Skip parts that look like file paths
		if !strings.Contains(part, "/") && !strings.Contains(part, "\\") {
			cleanParts = append(cleanParts, part)
		}
	}

	if len(cleanParts) > 0 {
		return strings.Join(cleanParts, " ")
	}

	return errorLine
}

// Disable/Enable web search (for testing or config)
func (ws *WebSearch) SetEnabled(enabled bool) {
	ws.enabled = enabled
}