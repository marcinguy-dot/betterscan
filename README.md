# Checkmate5 Monorepo

This repository contains the Go implementation of Checkmate:

- `go-checkmate/` – Go CLI runner (OpenGrep, Trivy, Bandit, Brakeman, staticcheck, Joern, Fraunhofer CPG)
- `checkmate-web/` – Self-hosted platform (API, worker, Next.js + jQuery UIs, Redis scan queue, VCS integrations)
- `fixtures/vulnerable/` – Small multi-language samples for CLI smoke tests
- `deploy/cloud-run-jobs/go-checkmate/` – Cloud Run Jobs templates (run N tasks)
- `deploy/ecs-fargate/go-checkmate/` – ECS/Fargate templates (run N tasks)
- `broken-vulnerable-code-snippets/` – Vulnerable code dataset (cloned from https://github.com/snoopysecurity/Broken-Vulnerable-Code-Snippets)
- `Speed.md` – Benchmark results

For CLI usage, see `go-checkmate/README.md`. For the web stack, see `checkmate-web/README.md`.

**Graph engines:** Checkmate runs **Joern** for mature, excellent default vulnerability queries, and **Fraunhofer CPG** primarily for **new custom rules** and **project-specific enforcement** on a multi-language code property graph (library / extensible analyses—aligned with Fraunhofer’s platform framing and tools such as Codyze). Details: [go-checkmate/README.md § Graph engines](go-checkmate/README.md#graph-engines-joern-vs-fraunhofer-cpg).

**Web VCS access:** Projects can link a **GitHub App** install (permissions + repo picker) or a **PAT/token** for GitLab, Bitbucket, GitHub, or generic HTTPS remotes. Scans are enqueued on Redis; the worker clones with short-lived credentials. Details: [checkmate-web/README.md § VCS](checkmate-web/README.md#vcs-github-app-vs-gitlab--bitbucket--other).

## Web Platform Frontends: Next.js vs jQuery

`checkmate-web/` provides a web UI for the platform with **two interchangeable
frontends** (same Go backend, same local email+password / JWT auth, same
Integrations + Projects + Scan Now flows). Both are
included on `main` and run side by side, so you can compare the look & feel and
choose one:

| Frontend | Stack | Port |
|---|---|---|
| Next.js | Next.js 16 + Tailwind + Base UI + Recharts | http://localhost:3000 |
| jQuery  | jQuery 3.7 + Bootstrap 5 + Chart.js (zero-build) | http://localhost:8081 |

```bash
cd checkmate-web && docker compose up      # then open :3000 and/or :8081
```

### Preview the look & feel before installing

Login and dashboard, side by side (full screenshots of every screen are in the
[comparison doc](checkmate-web/docs/comparison/README.md)):

| | Next.js | jQuery |
|---|---|---|
| Login | [![Next.js login](checkmate-web/docs/comparison/screenshots/nextjs/01-login.png)](checkmate-web/docs/comparison/screenshots/nextjs/01-login.png) | [![jQuery login](checkmate-web/docs/comparison/screenshots/jquery/01-login.png)](checkmate-web/docs/comparison/screenshots/jquery/01-login.png) |
| Dashboard | [![Next.js dashboard](checkmate-web/docs/comparison/screenshots/nextjs/02-dashboard.png)](checkmate-web/docs/comparison/screenshots/nextjs/02-dashboard.png) | [![jQuery dashboard](checkmate-web/docs/comparison/screenshots/jquery/02-dashboard.png)](checkmate-web/docs/comparison/screenshots/jquery/02-dashboard.png) |

**Full side-by-side comparison (login, dashboard, trends, projects, register):**
[`checkmate-web/docs/comparison/README.md`](checkmate-web/docs/comparison/README.md)

**Trade-off:** Next.js gives a nicer developer experience and a more modern look
but needs ongoing maintenance (npm audit churn, build, upgrades); the jQuery
version is plainer but zero-build and will run for years untouched. The two are
also available as single-frontend branches
([`frontend-nextjs`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-nextjs),
[`frontend-jquery`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-jquery))
if you want to deploy only one.

## Security & Isolation: Qubes OS for Workers

When scanning untrusted source code, container escape vulnerabilities in Docker (e.g. kernel exploits or runtime bugs in `runc`) pose a threat to the host system.

To mitigate this, Checkmate can run the worker component inside **Qubes OS** to isolate untrusted code execution using Xen-based hypervisor-level compartmentalization:

```mermaid
graph TD
    subgraph Qubes OS Host
        Dom0[Dom0: Admin Domain - Air-gapped / No Network]

        subgraph AppVM 1: Web Frontend & DB
            Frontend[Next.js App / Postgres]
        end

        subgraph AppVM 2: Worker Qube
            subgraph Docker Container
                Worker[Worker Running Untrusted Code]
            end
        end
    end

    Worker -.->|1. Container Escape| AppVM 2
    AppVM 2 -->|2. Trapped by Xen Hypervisor| Dom0
```

Even if malicious code manages to exploit a vulnerability and escape the Docker container, the attacker remains trapped inside the isolated VM boundary, protecting your host, database, and other components from being compromised.

For detailed setup instructions on making Docker storage persistent in Qubes OS using `bind-dirs` or shifting the `data-root`, see [QUBES_OS_DOCKER.md](QUBES_OS_DOCKER.md).

