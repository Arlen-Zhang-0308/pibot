package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	duckduckgoAPIURL   = "https://api.duckduckgo.com/"
	defaultMaxResults  = 5
	httpTimeoutSeconds = 15
)

// WebSearchTool searches the web using the DuckDuckGo Instant Answer API.
// No API key is required.
type WebSearchTool struct {
	httpClient *http.Client
}

// NewWebSearchTool creates a new WebSearchTool.
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		httpClient: &http.Client{
			Timeout: httpTimeoutSeconds * time.Second,
		},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web for information using DuckDuckGo. Returns an abstract summary, direct answer (if available), and related topics for the query."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query to look up on the web.",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of related topics to include in the results (default: 5, max: 10).",
				"default":     defaultMaxResults,
			},
		},
		"required": []string{"query"},
	}
}

// duckduckgoResponse is the relevant subset of the DuckDuckGo Instant Answer JSON response.
type duckduckgoResponse struct {
	Abstract       string `json:"Abstract"`
	AbstractSource string `json:"AbstractSource"`
	AbstractURL    string `json:"AbstractURL"`
	Answer         string `json:"Answer"`
	AnswerType     string `json:"AnswerType"`
	RelatedTopics  []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
		Topics   []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Topics"`
	} `json:"RelatedTopics"`
}

type webSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func (t *WebSearchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p webSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if strings.TrimSpace(p.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if p.MaxResults <= 0 {
		p.MaxResults = defaultMaxResults
	}
	if p.MaxResults > 10 {
		p.MaxResults = 10
	}

	apiURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1&skip_disambig=1",
		duckduckgoAPIURL, url.QueryEscape(p.Query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "PiBot/1.0 (https://github.com/pibot/pibot)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ddgResp duckduckgoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return t.formatResults(p.Query, &ddgResp, p.MaxResults), nil
}

func (t *WebSearchTool) formatResults(query string, r *duckduckgoResponse, maxResults int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Search results for: %q\n\n", query))

	if r.Answer != "" {
		sb.WriteString(fmt.Sprintf("Direct Answer (%s):\n%s\n\n", r.AnswerType, r.Answer))
	}

	if r.Abstract != "" {
		sb.WriteString(fmt.Sprintf("Summary (from %s):\n%s\n", r.AbstractSource, r.Abstract))
		if r.AbstractURL != "" {
			sb.WriteString(fmt.Sprintf("Source: %s\n", r.AbstractURL))
		}
		sb.WriteString("\n")
	}

	// Collect flat list of related topics (topics may be nested under a group)
	type topic struct {
		text string
		url  string
	}
	var topics []topic
	for _, rt := range r.RelatedTopics {
		if rt.Text != "" {
			topics = append(topics, topic{text: rt.Text, url: rt.FirstURL})
		}
		for _, sub := range rt.Topics {
			if sub.Text != "" {
				topics = append(topics, topic{text: sub.Text, url: sub.FirstURL})
			}
		}
	}

	if len(topics) > 0 {
		sb.WriteString("Related Topics:\n")
		limit := maxResults
		if limit > len(topics) {
			limit = len(topics)
		}
		for i, tp := range topics[:limit] {
			sb.WriteString(fmt.Sprintf("%d. %s", i+1, tp.text))
			if tp.url != "" {
				sb.WriteString(fmt.Sprintf("\n   %s", tp.url))
			}
			sb.WriteString("\n")
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == fmt.Sprintf("Search results for: %q", query) {
		return fmt.Sprintf("No results found for %q. Try rephrasing your query.", query)
	}
	return result
}
