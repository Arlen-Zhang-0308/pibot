package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pibot/pibot/internal/config"
)

const (
	duckduckgoAPIURL   = "https://api.duckduckgo.com/"
	perplexitySearchURL = "https://api.perplexity.ai/search"
	defaultMaxResults  = 5
	httpTimeoutSeconds = 15
)

// WebSearchTool searches the web, preferring DuckDuckGo and falling back to
// Perplexity when a DuckDuckGo API key is not configured.
type WebSearchTool struct {
	cfg        config.WebSearchConfig
	httpClient *http.Client
}

// NewWebSearchTool creates a new WebSearchTool using the provided config.
func NewWebSearchTool(cfg config.WebSearchConfig) *WebSearchTool {
	return &WebSearchTool{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: httpTimeoutSeconds * time.Second,
		},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web for information. Uses DuckDuckGo by default; falls back to Perplexity when configured."
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
				"description": "Maximum number of results to return (default: 5, max: 10).",
				"default":     defaultMaxResults,
			},
		},
		"required": []string{"query"},
	}
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

	// DuckDuckGo is the default (free, no key required). If its api_key is set,
	// it is forwarded as a Bearer token. Perplexity is used only when the DDG
	// api_key is empty AND a Perplexity api_key is configured.
	if t.cfg.DuckDuckGoAPIKey != "" || t.cfg.PerplexityAPIKey == "" {
		return t.duckduckgoSearch(ctx, p)
	}
	return t.perplexitySearch(ctx, p)
}

// ── DuckDuckGo ────────────────────────────────────────────────────────────────

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

func (t *WebSearchTool) duckduckgoSearch(ctx context.Context, p webSearchParams) (string, error) {
	apiURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1&skip_disambig=1",
		duckduckgoAPIURL, url.QueryEscape(p.Query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "PiBot/1.0 (https://github.com/pibot/pibot)")
	if t.cfg.DuckDuckGoAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.cfg.DuckDuckGoAPIKey)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DuckDuckGo API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ddgResp duckduckgoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return t.formatDDGResults(p.Query, &ddgResp, p.MaxResults), nil
}

func (t *WebSearchTool) formatDDGResults(query string, r *duckduckgoResponse, maxResults int) string {
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

// ── Perplexity Search API ─────────────────────────────────────────────────────

type perplexitySearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type perplexitySearchResponse struct {
	Results []perplexitySearchResult `json:"results"`
}

type perplexitySearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Date    string `json:"date"`
}

func (t *WebSearchTool) perplexitySearch(ctx context.Context, p webSearchParams) (string, error) {
	reqBody := perplexitySearchRequest{
		Query:      p.Query,
		MaxResults: p.MaxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to build request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, perplexitySearchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.cfg.PerplexityAPIKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("perplexity search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ = io.ReadAll(resp.Body)
		return "", fmt.Errorf("perplexity API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read perplexity response: %w", err)
	}

	var pResp perplexitySearchResponse
	if err := json.Unmarshal(bodyBytes, &pResp); err != nil {
		return "", fmt.Errorf("failed to parse perplexity response: %w", err)
	}

	return t.formatPerplexityResults(p.Query, pResp.Results), nil
}

func (t *WebSearchTool) formatPerplexityResults(query string, results []perplexitySearchResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("No results found for %q. Try rephrasing your query.", query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %q\n\n", query))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   %s\n", r.URL))
		if r.Date != "" {
			sb.WriteString(fmt.Sprintf("   Date: %s\n", r.Date))
		}
		if r.Snippet != "" {
			// Trim very long snippets to keep the output readable
			snippet := r.Snippet
			if len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}
