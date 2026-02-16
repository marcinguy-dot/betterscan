 # Checkmate Go
 
Go-based scanner runner with sequential/parallel execution, normalized findings,
and optional LLM enrichment for JSON/SARIF/HTML outputs.
 
## Features
- Runs common Checkmate tools (OpenGrep, Trivy, Bandit, Brakeman, GoStaticcheck)
- Two strategies:
  - `sequential`: run tools one-by-one with internal tool jobs
  - `parallel`: run tools concurrently with a core-limited worker pool
- LLM enrichment (behind `--llm-enrich`) with model selection and swappable OpenAI-compatible endpoint
- Output formats: JSON summary (`--json-out`), SARIF (`--sarif-out`), HTML (`--html-out`)
- Passes `-j` to OpenGrep
- Refreshes OpenGrep Aikido + Amplify rules before scan (by default)
 
 ## Build
 
 ```bash
 go build -o checkmate-go .
 ```
 
 ## Usage
 
 ```bash
./checkmate-go --code-dir /path/to/project --strategy parallel --jobs 8
 ```

Install missing tools automatically (default on):

```bash
./checkmate-go --install-missing
```
 
 Run both strategies and write JSON summary:
 
 ```bash
 ./checkmate-go --code-dir /path/to/project --strategy both --jobs 8 --json-out results.json
 ```

Generate all output formats with LLM enrichment:

```bash
./checkmate-go \
  --code-dir /path/to/project \
  --strategy parallel \
  --jobs 12 \
  --json-out results.json \
  --sarif-out results.sarif \
  --html-out results.html \
  --llm-enrich \
  --llm-provider openai \
  --llm-model gpt-4.1-mini \
  --llm-endpoint https://api.openai.com/v1/chat/completions
```
 
 Use explicit rules paths (skip refresh):
 
 ```bash
 ./checkmate-go \
   --code-dir /path/to/project \
   --rules-aikido "$HOME/.opengrep/rules/aikido-rules/rules" \
   --rules-amplify "$HOME/.opengrep/rules/amplify-rules/rules" \
   --no-refresh
 ```
 
 Restrict tools (comma-separated):
 
```bash
./checkmate-go --tools opengrep,trivy,gostaticcheck --code-dir /path/to/project
```

Scan the Broken-Vulnerable-Code-Snippets dataset (cloned locally):

```bash
./checkmate-go --code-dir ../broken-vulnerable-code-snippets --strategy both --jobs 12 --tools opengrep,gostaticcheck
```

## Cloud deployment helpers
- Cloud Run Jobs templates/scripts: `../deploy/cloud-run-jobs/go-checkmate/`
- ECS/Fargate templates/scripts: `../deploy/ecs-fargate/go-checkmate/`

## Normalization and deduplication

Go-checkmate parses tool JSON outputs and normalizes findings into a unified
issue format (`analyzer`, `code`, `file`, `line`, `message`, `fingerprint`).
By default, issues are de-duplicated using `analyzer + code + fingerprint`.
You can optionally collapse findings across tools on the same `file:line`:

```bash
./checkmate-go --dedupe-scope file-line --code-dir /path/to/project
```

When multiple findings collapse into one, the retained entry is the one with
the longest message.

 ## Tool parallel support
 - OpenGrep: `-j <jobs>`
 - Others: no documented parallel flag; run as-is
 
## LLM enrichment flags
- `--llm-enrich`: enable enrichment
- `--llm-model`: model to use (required when enrichment is enabled)
- `--llm-provider`: provider label for metadata (default: `openai`)
- `--llm-endpoint`: OpenAI-compatible chat completions endpoint
- `--llm-api-key`: API key (or set `OPENAI_API_KEY`)
- `--llm-workers`: parallel LLM requests (default: `4`)
- `--llm-max-issues`: max findings to enrich (default: `100`)
- `--llm-timeout`: per-request timeout in seconds (default: `30`)

## Notes
- Tools are executed if their binaries are available in `PATH`.
- Default strategy is `parallel`.
