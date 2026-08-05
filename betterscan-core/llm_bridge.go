package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type IssueEnrichment struct {
	Summary       string   `json:"summary"`
	OWASPCategory string   `json:"owasp_category"`
	Mitigations   []string `json:"mitigations"`
	FixSuggestion string   `json:"fix_suggestion"`
	Confidence    string   `json:"confidence"`
	Model         string   `json:"model,omitempty"`
	Provider      string   `json:"provider,omitempty"`
}

type LLMConfig struct {
	Provider  string
	Model     string
	Endpoint  string
	APIKey    string
	Workers   int
	MaxIssues int
	Timeout   time.Duration
}

type openAICompatibleBridge struct {
	config LLMConfig
	client *http.Client
}

func newLLMBridge(config LLMConfig) (*openAICompatibleBridge, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("llm-model is required when --llm-enrich is enabled")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("llm-api-key or OPENAI_API_KEY is required when --llm-enrich is enabled")
	}
	if config.Workers < 1 {
		config.Workers = 1
	}
	if config.MaxIssues < 1 {
		config.MaxIssues = 1
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errors.New("llm-endpoint is required when --llm-enrich is enabled")
	}
	return &openAICompatibleBridge{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}, nil
}

func enrichIssuesWithLLM(ctx context.Context, issues []Issue, config LLMConfig) ([]Issue, error) {
	bridge, err := newLLMBridge(config)
	if err != nil {
		return issues, err
	}
	limit := len(issues)
	if config.MaxIssues < limit {
		limit = config.MaxIssues
	}
	if limit == 0 {
		return issues, nil
	}

	enriched := make([]Issue, len(issues))
	copy(enriched, issues)

	sem := make(chan struct{}, config.Workers)
	var wg sync.WaitGroup
	errs := make(chan error, limit)

	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			issueCtx, cancel := context.WithTimeout(ctx, config.Timeout)
			defer cancel()
			enrichment, enrichErr := bridge.enrichIssue(issueCtx, enriched[idx])
			if enrichErr != nil {
				errs <- enrichErr
				return
			}
			enriched[idx].Enrichment = enrichment
		}(i)
	}
	wg.Wait()
	close(errs)

	// Non-fatal: return enriched issues and first encountered error as warning context.
	for enrichErr := range errs {
		return enriched, enrichErr
	}
	return enriched, nil
}

func (b *openAICompatibleBridge) enrichIssue(ctx context.Context, issue Issue) (*IssueEnrichment, error) {
	reqBody := map[string]any{
		"model": b.config.Model,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "You are a secure code reviewer. Use OWASP Top 10 (2021) guidance. " +
					"Return only strict JSON with keys: summary, owasp_category, mitigations, fix_suggestion, confidence.",
			},
			{
				"role":    "user",
				"content": buildOWASPPrompt(issue),
			},
		},
		"temperature": 0.2,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.config.APIKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm request failed with status %s", resp.Status)
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return nil, err
	}
	if len(llmResp.Choices) == 0 {
		return nil, errors.New("llm response has no choices")
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var enrichment IssueEnrichment
	if err := json.Unmarshal([]byte(content), &enrichment); err != nil {
		return nil, fmt.Errorf("failed to parse llm enrichment json: %w", err)
	}
	enrichment.Model = b.config.Model
	enrichment.Provider = b.config.Provider
	return &enrichment, nil
}

func buildOWASPPrompt(issue Issue) string {
	return fmt.Sprintf(
		"Issue details:\n- analyzer: %s\n- code: %s\n- file: %s\n- line: %d\n- message: %s\n\n"+
			"Tasks:\n"+
			"1) Summarize the finding in 2-3 sentences.\n"+
			"2) Map it to OWASP Top 10 2021 category (A01..A10) with short rationale.\n"+
			"3) Provide concrete mitigations as a list.\n"+
			"4) Provide a possible code fix suggestion (safe pseudocode or snippet guidance).\n"+
			"5) Set confidence as low|medium|high.\n\n"+
			"Output JSON only.",
		issue.Analyzer, issue.Code, issue.File, issue.Line, issue.Message,
	)
}
