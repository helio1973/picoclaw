package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// DeepScrapeTool scrapes web pages using a headless browser backend (Crawl4AI).
// It returns clean, LLM-ready markdown and supports both single-page and deep crawl modes.
type DeepScrapeTool struct {
	baseURL      string
	client       *http.Client
	pollInterval time.Duration
}

// NewDeepScrapeTool creates a new DeepScrapeTool pointing at the given Crawl4AI base URL.
// If timeoutSecs is <= 0, a default of 120 seconds is used.
func NewDeepScrapeTool(baseURL string, timeoutSecs int) *DeepScrapeTool {
	if timeoutSecs <= 0 {
		timeoutSecs = 120
	}
	return &DeepScrapeTool{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
		pollInterval: 2 * time.Second,
	}
}

// Name returns the tool name.
func (t *DeepScrapeTool) Name() string { return "deep_scrape" }

// Description returns a human-readable description of the tool.
func (t *DeepScrapeTool) Description() string {
	return "Scrape web pages using a headless browser (Crawl4AI). Returns clean, LLM-ready markdown. Use for JavaScript-heavy sites, SPAs, and dynamic content that web_fetch cannot handle."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *DeepScrapeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to scrape",
			},
			"max_depth": map[string]any{
				"type":        "integer",
				"description": "Link depth to follow (default 1, max 5)",
			},
			"max_pages": map[string]any{
				"type":        "integer",
				"description": "Max pages to crawl (default 1, max 50)",
			},
		},
		"required": []string{"url"},
	}
}

// Execute runs the deep scrape operation.
func (t *DeepScrapeTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Extract url (required)
	urlStr, _ := args["url"].(string)
	if urlStr == "" {
		return ErrorResult("url is required")
	}

	// Extract optional parameters with defaults
	maxDepth := intFromArgs(args, "max_depth", 1)
	maxPages := intFromArgs(args, "max_pages", 1)

	// Clamp values
	if maxDepth > 5 {
		maxDepth = 5
	}
	if maxPages > 50 {
		maxPages = 50
	}

	// Single page mode vs deep crawl mode
	if maxDepth <= 1 && maxPages <= 1 {
		return t.singlePageCrawl(ctx, urlStr)
	}
	return t.deepCrawl(ctx, urlStr, maxDepth, maxPages)
}

// singlePageCrawl performs a synchronous single-page scrape via POST /crawl.
func (t *DeepScrapeTool) singlePageCrawl(ctx context.Context, urlStr string) *ToolResult {
	payload := map[string]any{
		"urls": []string{urlStr},
		"crawler_config": map[string]any{
			"type": "CrawlerRunConfig",
			"params": map[string]any{
				"cache_mode": "bypass",
			},
		},
	}

	body, err := t.postJSON(ctx, t.baseURL+"/crawl", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("crawl request failed: %v", err))
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse crawl response: %v", err))
	}

	markdown := extractMarkdownFromResults(resp)
	if markdown == "" {
		return ErrorResult("no content returned from crawl")
	}

	return &ToolResult{
		ForLLM:  markdown,
		ForUser: fmt.Sprintf("Scraped %s (%d chars)", urlStr, len(markdown)),
	}
}

// deepCrawl performs an async deep crawl via POST /crawl/job and polls for completion.
func (t *DeepScrapeTool) deepCrawl(ctx context.Context, urlStr string, maxDepth, maxPages int) *ToolResult {
	payload := map[string]any{
		"urls": []string{urlStr},
		"crawler_config": map[string]any{
			"type": "CrawlerRunConfig",
			"params": map[string]any{
				"cache_mode": "bypass",
			},
		},
		"deep_crawl_strategy": map[string]any{
			"type": "BFSDeepCrawlStrategy",
			"params": map[string]any{
				"max_depth": maxDepth,
				"max_pages": maxPages,
			},
		},
	}

	body, err := t.postJSON(ctx, t.baseURL+"/crawl/job", payload)
	if err != nil {
		return ErrorResult(fmt.Sprintf("deep crawl job request failed: %v", err))
	}

	var jobResp map[string]any
	if err := json.Unmarshal(body, &jobResp); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse job response: %v", err))
	}

	taskID, _ := jobResp["task_id"].(string)
	if taskID == "" {
		return ErrorResult("no task_id returned from deep crawl job")
	}

	// Poll for completion
	pollURL := fmt.Sprintf("%s/job/%s", t.baseURL, taskID)
	for {
		select {
		case <-ctx.Done():
			return ErrorResult(fmt.Sprintf("deep crawl cancelled: %v", ctx.Err()))
		case <-time.After(t.pollInterval):
		}

		statusBody, err := t.getJSON(ctx, pollURL)
		if err != nil {
			// Context cancellation during HTTP request
			if ctx.Err() != nil {
				return ErrorResult(fmt.Sprintf("deep crawl cancelled: %v", ctx.Err()))
			}
			return ErrorResult(fmt.Sprintf("failed to poll job status: %v", err))
		}

		var statusResp map[string]any
		if err := json.Unmarshal(statusBody, &statusResp); err != nil {
			return ErrorResult(fmt.Sprintf("failed to parse job status: %v", err))
		}

		status, _ := statusResp["status"].(string)
		switch status {
		case "completed":
			markdown := extractMarkdownFromResults(statusResp)
			if markdown == "" {
				return ErrorResult("deep crawl completed but no content returned")
			}
			return &ToolResult{
				ForLLM:  markdown,
				ForUser: fmt.Sprintf("Scraped %s (%d chars)", urlStr, len(markdown)),
			}
		case "failed":
			errMsg, _ := statusResp["error"].(string)
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return ErrorResult(fmt.Sprintf("deep crawl failed: %s", errMsg))
		}
		// Otherwise keep polling (pending, running, etc.)
	}
}

// postJSON sends a POST request with JSON body and returns the response body.
func (t *DeepScrapeTool) postJSON(ctx context.Context, url string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// getJSON sends a GET request and returns the response body.
func (t *DeepScrapeTool) getJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// extractMarkdownFromResult extracts markdown from a single result object.
func extractMarkdownFromResult(result map[string]any) string {
	// Try fit_markdown first (cleaner), then markdown, then markdown_v2.fit_markdown
	if md, _ := result["fit_markdown"].(string); md != "" {
		return md
	}
	if md, _ := result["markdown"].(string); md != "" {
		return md
	}
	// Crawl4AI 0.8.x uses nested markdown_v2 structure
	if mdv2, ok := result["markdown_v2"].(map[string]any); ok {
		if md, _ := mdv2["fit_markdown"].(string); md != "" {
			return md
		}
		if md, _ := mdv2["raw_markdown"].(string); md != "" {
			return md
		}
	}
	return ""
}

// extractMarkdownFromResults extracts markdown content from the Crawl4AI response.
// Handles multiple response formats across Crawl4AI versions:
//   - "results": [...]  (array of result objects)
//   - "result": {...}   (single result object)
//   - top-level markdown fields (flat response)
func extractMarkdownFromResults(resp map[string]any) string {
	// Format 1: "results" array (Crawl4AI <= 0.7.x)
	if results, ok := resp["results"].([]any); ok {
		var parts []string
		for _, r := range results {
			if result, ok := r.(map[string]any); ok {
				if md := extractMarkdownFromResult(result); md != "" {
					parts = append(parts, md)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n---\n\n")
		}
	}

	// Format 2: "result" singular object (Crawl4AI 0.8.x)
	if result, ok := resp["result"].(map[string]any); ok {
		if md := extractMarkdownFromResult(result); md != "" {
			return md
		}
	}

	// Format 3: top-level markdown fields (flat response)
	if md := extractMarkdownFromResult(resp); md != "" {
		return md
	}

	// Log the response keys for debugging when no content is found
	keys := make([]string, 0, len(resp))
	for k := range resp {
		keys = append(keys, k)
	}
	logger.WarnCF("deep_scrape", "No markdown found in response",
		map[string]any{"response_keys": keys})

	return ""
}

// intFromArgs extracts an integer from args with a default value.
func intFromArgs(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case json.Number:
		if n, err := val.Int64(); err == nil {
			return int(n)
		}
	}
	return defaultVal
}
