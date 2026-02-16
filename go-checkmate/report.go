package main

import (
	"encoding/json"
	"html/template"
	"os"
	"sort"
	"strings"
)

func writeSARIF(path string, issues []Issue) error {
	type sarifRule struct {
		ID                   string            `json:"id"`
		Name                 string            `json:"name"`
		ShortDescription     map[string]string `json:"shortDescription"`
		FullDescription      map[string]string `json:"fullDescription"`
		Help                 map[string]string `json:"help,omitempty"`
		DefaultConfiguration map[string]string `json:"defaultConfiguration,omitempty"`
		Properties           map[string]any    `json:"properties,omitempty"`
	}
	type sarifResult struct {
		RuleID     string            `json:"ruleId"`
		Message    map[string]string `json:"message"`
		Locations  []map[string]any  `json:"locations"`
		Properties map[string]any    `json:"properties,omitempty"`
	}
	rules := map[string]sarifRule{}
	results := make([]sarifResult, 0, len(issues))

	for _, issue := range issues {
		ruleID := issue.Analyzer + ":" + issue.Code
		if _, exists := rules[ruleID]; !exists {
			rule := sarifRule{
				ID:               ruleID,
				Name:             issue.Analyzer,
				ShortDescription: map[string]string{"text": issue.Message},
				FullDescription:  map[string]string{"text": issue.Message},
				DefaultConfiguration: map[string]string{
					"level": severityToLevel(issue.Message),
				},
				Properties: map[string]any{
					"tags": []string{"security"},
				},
			}
			if issue.Enrichment != nil {
				rule.Help = map[string]string{
					"markdown": buildEnrichmentMarkdown(issue.Enrichment),
					"text":     issue.Enrichment.Summary,
				}
				rule.Properties["owasp_category"] = issue.Enrichment.OWASPCategory
			}
			rules[ruleID] = rule
		}
		result := sarifResult{
			RuleID:  ruleID,
			Message: map[string]string{"text": issue.Message},
			Locations: []map[string]any{
				{
					"physicalLocation": map[string]any{
						"artifactLocation": map[string]any{"uri": issue.File},
						"region":           map[string]any{"startLine": issue.Line},
					},
				},
			},
		}
		if issue.Enrichment != nil {
			result.Properties = map[string]any{
				"owasp_category": issue.Enrichment.OWASPCategory,
				"mitigations":    issue.Enrichment.Mitigations,
				"fix_suggestion": issue.Enrichment.FixSuggestion,
				"confidence":     issue.Enrichment.Confidence,
			}
		}
		results = append(results, result)
	}

	ruleList := make([]sarifRule, 0, len(rules))
	for _, rule := range rules {
		ruleList = append(ruleList, rule)
	}
	sort.Slice(ruleList, func(i, j int) bool { return ruleList[i].ID < ruleList[j].ID })

	sarif := map[string]any{
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{
			{
				"tool": map[string]any{
					"driver": map[string]any{
						"name":            "go-checkmate",
						"semanticVersion": "1.0.0",
						"rules":           ruleList,
					},
				},
				"results": results,
			},
		},
	}
	payload, err := json.MarshalIndent(sarif, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func writeHTML(path string, issues []Issue) error {
	const tpl = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>go-checkmate report</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 24px; }
    .issue { border: 1px solid #ddd; padding: 12px; margin-bottom: 12px; border-radius: 6px; }
    .meta { color: #444; font-size: 0.95em; margin-bottom: 8px; }
    ul { margin-top: 4px; }
  </style>
</head>
<body>
  <h1>go-checkmate Scan Report</h1>
  <p>Total findings: {{len .}}</p>
  {{range .}}
  <div class="issue">
    <div class="meta"><strong>{{.Analyzer}}</strong> | {{.Code}} | {{.File}}:{{.Line}}</div>
    <div><strong>Message:</strong> {{.Message}}</div>
    {{if .Enrichment}}
      <div><strong>LLM Summary:</strong> {{.Enrichment.Summary}}</div>
      <div><strong>OWASP:</strong> {{.Enrichment.OWASPCategory}}</div>
      <div><strong>Mitigations:</strong></div>
      <ul>{{range .Enrichment.Mitigations}}<li>{{.}}</li>{{end}}</ul>
      <div><strong>Possible fix:</strong> {{.Enrichment.FixSuggestion}}</div>
      <div><strong>Confidence:</strong> {{.Enrichment.Confidence}}</div>
    {{end}}
  </div>
  {{end}}
</body>
</html>`
	t, err := template.New("report").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, issues)
}

func buildEnrichmentMarkdown(enrichment *IssueEnrichment) string {
	lines := []string{
		"### LLM finding summary",
		enrichment.Summary,
		"",
		"**OWASP Top 10 category:** " + enrichment.OWASPCategory,
		"",
		"**Mitigations:**",
	}
	for _, mitigation := range enrichment.Mitigations {
		lines = append(lines, "- "+mitigation)
	}
	lines = append(lines, "", "**Possible fix suggestion:**", enrichment.FixSuggestion, "", "**Confidence:** "+enrichment.Confidence)
	return strings.Join(lines, "\n")
}

func severityToLevel(message string) string {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "critical") || strings.Contains(lower, "high") {
		return "error"
	}
	if strings.Contains(lower, "medium") || strings.Contains(lower, "warning") {
		return "warning"
	}
	return "note"
}
