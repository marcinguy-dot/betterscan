 # Checkmate5 Monorepo
 
This repository contains the Go implementation of Checkmate:
 
 - `go-checkmate/` – Go runner
- `deploy/cloud-run-jobs/go-checkmate/` – Cloud Run Jobs templates (run N tasks)
- `deploy/ecs-fargate/go-checkmate/` – ECS/Fargate templates (run N tasks)
- `broken-vulnerable-code-snippets/` – Vulnerable code dataset (cloned from https://github.com/snoopysecurity/Broken-Vulnerable-Code-Snippets)
 - `Speed.md` – Benchmark results
 
For usage and setup, see `go-checkmate/README.md`.

## Web Platform Frontends: Next.js vs jQuery

`checkmate-web/` provides a web UI for the platform. It comes in **two
interchangeable frontends** (same Go backend, same local email+password / JWT
auth) so you can compare the look & feel and choose one:

| Frontend | Branch | Stack | Port |
|---|---|---|---|
| Next.js | [`frontend-nextjs`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-nextjs) | Next.js 16 + Tailwind + Base UI + Recharts | 3000 |
| jQuery  | [`frontend-jquery`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-jquery) | jQuery 3.7 + Bootstrap 5 + Chart.js (zero-build) | 8081 |

Both pass the same end-to-end flow (login → dashboard → vulnerability trends →
projects → logout → register).

| | Next.js | jQuery |
|---|---|---|
| Styling | Tailwind + shadcn / Base UI | Bootstrap 5 |
| Look | flat, airy, lots of white space | denser, classic Bootstrap cards |
| Charts | Recharts (SVG) | Chart.js |
| Build | needs `npm` + build step + periodic upgrades | zero-build, vendored, runs untouched for years |
| Type safety / DX | TypeScript, components, hot reload | plain JS, manual DOM |
| Behavior | identical feature set & flow | identical feature set & flow |

**Trade-off:** Next.js gives a nicer developer experience and a more modern look
but needs ongoing maintenance (npm audit churn, build, upgrades); the jQuery
version is plainer but zero-build and will run for years untouched.

Detailed comparison with screenshots and the e2e test report:
[`checkmate-web/docs/comparison/`](checkmate-web/docs/comparison/README.md).
