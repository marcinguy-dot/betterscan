# Frontend Comparison: Next.js vs jQuery

BetterScan ships **two** interchangeable frontends that talk to the same Go
backend (same local email+password / JWT auth). Both pass the same end-to-end
flow. Pick whichever look & feel you prefer.

| | Next.js | jQuery |
|---|---|---|
| Stack | Next.js 16 + Tailwind + Base UI + Recharts | jQuery 3.7 + Bootstrap 5 + Chart.js |
| Port | http://localhost:3000 | http://localhost:8081 |
| Build | `npm` install + build step, periodic upgrades | zero-build, vendored, runs untouched for years |
| Look | flat, airy, lots of white space | denser, classic Bootstrap cards |
| Logout | User dropdown menu in the header | Log out button in the nav bar |

Run both side by side:

```bash
cd betterscan-web && docker compose up
```

Then open Next.js on :3000 and jQuery on :8081. Screens are login, register,
dashboard (stats + severity), projects, and scan results — compare them live
after you create a project and run a scan.

**Trade-off:** Next.js is a better long-term product UI if you accept npm/build
churn; jQuery is the “set and forget” option.
