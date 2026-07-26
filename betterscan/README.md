# BetterScan CLI

Go-based scanner runner with sequential/parallel execution, normalized findings,
and optional LLM enrichment for JSON/SARIF/HTML outputs.

BetterScan continues the **Contender** project line presented at
**Black Hat MEA 2025** in Saudi Arabia—rebuilt around multi-engine orchestration and
code property graphs.

## Features
- Runs common BetterScan tools (OpenGrep, Trivy, Bandit, Brakeman, GoStaticcheck, Joern, Fraunhofer CPG)
- Two strategies:
  - `sequential`: run tools one-by-one with internal tool jobs
  - `parallel`: run tools concurrently with a core-limited worker pool
- LLM enrichment (behind `--llm-enrich`) with model selection and swappable OpenAI-compatible endpoint
- Output formats: JSON summary (`--json-out`), SARIF (`--sarif-out`), HTML (`--html-out`)
- Passes `-j` to OpenGrep
- Refreshes OpenGrep Aikido + Amplify rules before scan (by default)

## Graph engines: Joern vs Fraunhofer CPG

BetterScan wires **two** code-property-graph style engines. They are complementary, not duplicates:

| Engine | Role in BetterScan | Strength |
|--------|-------------------|----------|
| **[Joern](https://joern.io/)** | Default depth for known vuln patterns | Large, mature **query / rule pack** (querydb and community queries) for classic issues (e.g. `gets`, taint-style sinks). Prefer Joern when you want excellent out-of-the-box graph queries. |
| **[Fraunhofer CPG](https://github.com/Fraunhofer-AISEC/cpg)** | Custom rules & project enforcement | A **library-first** CPG ([docs](https://fraunhofer-aisec.github.io/cpg/)): multi-language graph (AST/CFG/DFG/…), extensible frontends, passes, and analyses. Suited to **writing new checks** and **enforcing project- or org-specific policies** (similar spirit to Fraunhofer’s own [Codyze](https://fraunhofer-aisec.github.io/cpg/GettingStarted/codyze/) compliance tooling), not to shipping Joern-scale default rule volume. |

**Practical guidance**

- Use **Joern** for broad, battle-tested graph findings.
- Use **CPG** when you need *new* semantic rules, compliance-style goals, or checks tailored to your codebase—built on Fraunhofer’s graph rather than pattern packs alone.
- Findings can still land on the same `file:line`; with `--dedupe-scope file-line`, BetterScan collapses cross-tool duplicates and keeps the longest message.

Fraunhofer positions the CPG as a language-independent analysis **platform** (query API, Neo4j export, library embed, compliance via Codyze)—highly extensible with custom language frontends, passes, and analyses—rather than a single fixed “scanner ruleset.” BetterScan’s `cpg` integration follows that model: a small built-in starter suite (e.g. NPE / OOB) plus room to grow **your** rules.

## Build

```bash
go build -o betterscan .
```

## Usage

```bash
./betterscan --code-dir /path/to/project --strategy parallel --jobs 8
```

Install missing tools automatically (default on):

```bash
./betterscan --install-missing
```

Run both strategies and write JSON summary:

```bash
./betterscan --code-dir /path/to/project --strategy both --jobs 8 --json-out results.json
```

Generate all output formats with LLM enrichment:

```bash
./betterscan \
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
./betterscan \
  --code-dir /path/to/project \
  --rules-aikido "$HOME/.opengrep/rules/aikido-rules/rules" \
  --rules-amplify "$HOME/.opengrep/rules/amplify-rules/rules" \
  --no-refresh
```

Restrict tools (comma-separated):

```bash
./betterscan --tools opengrep,trivy,gostaticcheck --code-dir /path/to/project
```

Run Fraunhofer CPG (requires Java 17+; auto-installs JDK and CPG runtime when `--install-missing` is enabled):

```bash
./betterscan --tools cpg --code-dir /path/to/project
```

Scan the Broken-Vulnerable-Code-Snippets dataset (cloned locally):

```bash
./betterscan --code-dir ../broken-vulnerable-code-snippets --strategy both --jobs 12 --tools opengrep,gostaticcheck
```

## Cloud deployment helpers
- Cloud Run Jobs templates/scripts: `../deploy/cloud-run-jobs/betterscan/`
- ECS/Fargate templates/scripts: `../deploy/ecs-fargate/betterscan/`

## Normalization and deduplication

BetterScan parses tool JSON outputs and normalizes findings into a unified
issue format (`analyzer`, `code`, `file`, `line`, `message`, `fingerprint`).
By default, issues are de-duplicated using `analyzer + code + fingerprint`.
You can optionally collapse findings across tools on the same `file:line`:

```bash
./betterscan --dedupe-scope file-line --code-dir /path/to/project
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

## Joern integration

The `joern` tool runs [Joern](https://joern.io/) scan queries against your code directory (auto-install under `~/.betterscan/joern` when `--install-missing` is set). Prefer it for **excellent existing graph rules** and known vulnerability query packs.

```bash
./betterscan --tools joern --code-dir /path/to/project
```

## Fraunhofer CPG integration

The `cpg` tool wraps [Fraunhofer CPG](https://github.com/Fraunhofer-AISEC/cpg). Source detection covers common extensions (Java, C/C++, Go, Python, Ruby, TypeScript/JavaScript, LLVM IR, INI); the bundled Runner currently focuses on starter semantic checks (null-pointer and out-of-bounds) as a base for **custom / project-specific** enforcement.

Use CPG when you want to **add or enforce new rules** on the Fraunhofer graph (library + extensible analyses), not as a replacement for Joern’s large default query set. See [Graph engines: Joern vs Fraunhofer CPG](#graph-engines-joern-vs-fraunhofer-cpg) above.

- Requires **Java 17+**. If Java is missing and `--install-missing` is enabled, betterscan downloads an Eclipse Temurin JDK for the current OS/architecture (`darwin`/`linux`/`windows`, `amd64`/`arm64`).
- Downloads CPG `10.8.2` dependencies from Maven Central into `~/.betterscan/cpg/10.8.2/`.
- If Java cannot be installed automatically (unsupported OS/arch) or dependency setup fails, the tool reports a clear error and skips the scan.

## Notes
- Tools are executed if their binaries are available in `PATH` (or auto-installed when supported).
- Default strategy is `parallel`.
