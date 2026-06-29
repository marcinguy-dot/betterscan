 # Checkmate5 Monorepo
 
This repository contains the Go implementation of Checkmate:
 
 - `go-checkmate/` – Go runner
- `deploy/cloud-run-jobs/go-checkmate/` – Cloud Run Jobs templates (run N tasks)
- `deploy/ecs-fargate/go-checkmate/` – ECS/Fargate templates (run N tasks)
- `broken-vulnerable-code-snippets/` – Vulnerable code dataset (cloned from https://github.com/snoopysecurity/Broken-Vulnerable-Code-Snippets)
 - `Speed.md` – Benchmark results
 
For usage and setup, see `go-checkmate/README.md`.

## Web Platform Frontends: Next.js vs jQuery

`checkmate-web/` provides a web UI for the platform with **two interchangeable
frontends** (same Go backend, same local email+password / JWT auth). Both are
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
