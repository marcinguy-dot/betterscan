# Lattice jQuery frontend (zero-build alternative)

A plain static frontend for Lattice built with **jQuery + Bootstrap** and no
build tooling. It is an alternative to the Next.js app in `../frontend`, intended
for comparing look & feel and long-term maintenance burden.

Everything is vendored locally (`vendor/`), so there is no `npm install`, no
bundler, and nothing that needs upgrading on a regular cadence.

## Pages

| File            | Purpose                                  |
| --------------- | ---------------------------------------- |
| `login.html`    | Email + password sign in                 |
| `register.html` | Create an account                        |
| `index.html`    | Security dashboard (stats, recent scans) |
| `projects.html` | List and create projects                 |

## How auth works

It talks to the **same Go backend** as the Next.js app:

1. `login.html` / `register.html` POST to `/api/v1/auth/login` and
   `/api/v1/auth/register`.
2. The returned JWT is stored in `localStorage` (`lattice_token`).
3. `js/app.js` attaches `Authorization: Bearer <token>` to every API call and
   redirects to the login page on a `401`.

## Configuration

Point the site at a backend by editing the single value in `js/config.js`:

```js
window.LATTICE_API = "http://localhost:8080";
```

The backend must allow this site's origin via CORS. `docker-compose.yml` serves
this frontend on <http://localhost:8081>, which the backend allows by default
(see `CORS_ORIGINS`).

## Running

### With docker-compose (recommended)

```bash
docker compose up frontend-jquery
# open http://localhost:8081
```

### Standalone

Serve the directory with any static file server, e.g.:

```bash
python3 -m http.server 8081
# open http://localhost:8081
```

> Opening the files directly via `file://` will not work for API calls because
> of browser CORS rules; serve over HTTP instead.
