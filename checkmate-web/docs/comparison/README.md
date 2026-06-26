# Frontend Comparison: Next.js vs jQuery

Checkmate has **two** interchangeable frontends that talk to the same Go backend
(same local email+password / JWT auth). They live on separate branches so you
can run, compare, and pick the one you prefer.

| Frontend | Branch | Stack | Port |
|---|---|---|---|
| Next.js | [`frontend-nextjs`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-nextjs) | Next.js 16 + Tailwind + Base UI + Recharts | 3000 |
| jQuery  | [`frontend-jquery`](https://codeberg.org/marcinguy/checkmate-go/src/branch/frontend-jquery) | jQuery 3.7 + Bootstrap 5 + Chart.js (zero-build) | 8081 |

## Identical end-to-end flow (both pass)

1. Log in with email + password
2. Dashboard: stats cards + severity breakdown
3. Vulnerability Trends charts
4. Projects list
5. Log out (avatar dropdown / menu)
6. Register a new account → auto-login

Full Next.js report with screenshots: [nextjs-e2e-test-report.md](nextjs-e2e-test-report.md)

## Look & feel

| | Next.js | jQuery |
|---|---|---|
| Styling | Tailwind + shadcn / Base UI | Bootstrap 5 |
| Look | flat, airy, lots of white space | denser, classic Bootstrap cards |
| Charts | Recharts (SVG) | Chart.js |
| Build | needs `npm` + build step + periodic upgrades | zero-build, vendored, runs untouched for years |
| Type safety / DX | TypeScript, components, hot reload | plain JS, manual DOM |
| Bundle | larger, SSR/routing features | ~250 KB vendored libs, static files |
| Behavior | identical feature set & flow | identical feature set & flow |

**Trade-off in one line:** Next.js gives a nicer developer experience and a more
modern look but needs ongoing maintenance (npm audit churn, build, upgrades);
the jQuery version is plainer but is zero-build and will run for years without
touching it.

## Screenshots

The `screenshots/` folder contains the Next.js evidence frames.
