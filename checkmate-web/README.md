# Checkmate Web Platform

A modern web interface for the Checkmate code scanning platform with SSO authentication, dashboard, and vulnerability management.

## Architecture

```
checkmate-web/
├── frontend/          # Next.js App Router + TypeScript + shadcn/ui (:3000)
├── frontend-jquery/   # Zero-build jQuery + Bootstrap UI (:8081)
├── backend/           # Go API (Gin): auth, projects, scans, VCS integrations
├── worker/            # Redis consumer: clone (auth optional) + go-checkmate
├── e2e/               # Playwright click-through tests (Next + jQuery)
├── docker-compose.yml # Postgres, Redis, API, worker, both frontends
├── DEPLOYMENT.md      # Deployment guide
└── .env.example       # JWT, VCS, GitHub App, CORS, etc.
```

## Features

- **Authentication**: Local email/password JWT; optional SSO via OAuth2/OIDC (Google, GitHub, Gluu)
- **VCS integrations**: GitHub App install (permissions + repo picker), plus PAT/token connections for **GitLab, Bitbucket, GitHub, and generic HTTPS git** — one worker clone path with ephemeral credentials
- **Dashboard**: Scan history, vulnerability trends, statistics
- **Project Management**: Add/remove projects (manual URL or from a connection)
- **Scan Execution**: Queue scans to Redis; worker clones (authenticated when needed) and runs go-checkmate
- **Vulnerability Tracking**: Track findings by severity (Critical, High, Medium, Low)
- **False Positive Management**: Mark and suppress findings
- **API Integration**: RESTful API for automation
- **Browser e2e**: Playwright click-through suite under `e2e/`
- **Multi-cloud Deployment**: Support for AWS ECS, Google Cloud Run, Azure Container Apps

## VCS: GitHub App vs GitLab / Bitbucket / other

All private clones use the same model: store a **connection** (encrypted secret), mint credentials when enqueueing a scan, worker rewrites `https://user:pass@host/...`.

| Provider | How to connect | Clone username |
|----------|----------------|----------------|
| **GitHub App** (preferred for GitHub) | Integrations → Install App (or `GITHUB_APP_MOCK=1`) | `x-access-token` + installation token |
| **GitHub PAT** | Integrations → Token form | `x-access-token` |
| **GitLab** (SaaS or self-hosted) | PAT / project access token + host | `oauth2` |
| **Bitbucket Cloud** | Access token / app password | `x-token-auth` |
| **Generic** | Host + token | `git` (configurable) |

GitHub App env: `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_APP_PRIVATE_KEY` (or `_PATH`), callback `CHECKMATE_PUBLIC_URL/api/v1/vcs/github/callback`. Local default: `GITHUB_APP_MOCK=1`.

## Tech Stack

### Frontend
- **Next.js 14** (App Router)
- **TypeScript**
- **shadcn/ui** + Radix UI
- **TailwindCSS**
- **Recharts** (visualization)
- **NextAuth.js** (authentication)

### Backend
- **Go 1.21+**
- **Gin** (web framework)
- **GORM** (ORM)
- **PostgreSQL** (database)
- **Redis** (job queue & cache)

### Worker
- **Go 1.21+**
- **go-checkmate** scanner integration
- **Redis** job queue processing

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Git (for cloning repositories)

### Local Development

```bash
# Clone the repository
git clone https://codeberg.org/marcinguy/checkmate-go.git
cd checkmate-web

# Copy environment variables
cp .env.example .env
# Edit .env with your configuration

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

Access:
- **Next.js frontend**: http://localhost:3000
- **jQuery frontend**: http://localhost:8081
- **Backend API**: http://localhost:8080
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379

### Browser e2e (Playwright)

With the stack running:

```bash
cd e2e
npm install
npx playwright install chromium
npm test
```

Specs cover register/login, projects, and mock GitHub App install (when `GITHUB_APP_MOCK=1`).

### Manual Development

#### Frontend
```bash
cd frontend
npm install
npm run dev
# Access at http://localhost:3000
```

#### Backend
```bash
cd backend
go mod download
go run main.go
# Access at http://localhost:8080
```

#### Worker
```bash
cd worker
go mod download
go run main.go
```

## API Endpoints

### Authentication (local JWT)
- `POST /api/v1/auth/register` - Create account (email + password)
- `POST /api/v1/auth/login` - Login → `{ token, user }`
- `GET /api/v1/auth/me` - Current user (Bearer)
- `POST /api/v1/auth/logout` - Logout

### VCS integrations (Bearer)
- `GET /api/v1/vcs/providers` - Which providers are configured / mock
- `GET|POST /api/v1/vcs/connections` - List / create PAT connections
- `DELETE /api/v1/vcs/connections/:id` - Disconnect
- `GET /api/v1/vcs/connections/:id/repos` - List remote repos
- `GET /api/v1/vcs/github/install-url` - GitHub App install URL (or mock)
- `POST /api/v1/vcs/github/finalize` - Persist installation after redirect
- `GET /api/v1/vcs/github/callback` - Browser callback from GitHub
- `POST /api/v1/vcs/github/webhooks` - App webhook stub

### Projects / scans / findings
- `GET|POST /api/v1/projects`, `GET|PUT|DELETE /api/v1/projects/:id`
- `GET|POST /api/v1/scans` — **POST enqueues Redis job** for the worker
- `GET /api/v1/scans/:id`, `GET /api/v1/scans/:id/findings`
- `GET /api/v1/findings`, `PUT /api/v1/findings/:id/false-positive`
- `GET|POST /api/v1/schedules`, `PUT|DELETE /api/v1/schedules/:id`
- `GET /api/v1/dashboard/stats`, `GET /api/v1/dashboard/trends`
- `GET /api/v1/health` — public

## Environment Variables

See `.env.example` for the full list. Important groups:

### Core
- `DATABASE_URL`, `REDIS_ADDR`, `PORT`, `JWT_SECRET`, `CORS_ORIGINS`

### VCS / GitHub App
- `VCS_SECRET_KEY` — encrypt PATs at rest
- `GITHUB_APP_MOCK=1` — local/e2e without a real App
- `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_APP_PRIVATE_KEY` (or `_PATH`)
- `GITHUB_APP_CLIENT_ID` / `SECRET`, `GITHUB_APP_WEBHOOK_SECRET`
- `CHECKMATE_PUBLIC_URL`, `CHECKMATE_UI_URL` — callback and post-install redirects

### Frontends (NextAuth / optional SSO)
- `NEXT_PUBLIC_API_URL`, `INTERNAL_API_URL`, `NEXTAUTH_URL`, `NEXTAUTH_SECRET`
- `GOOGLE_CLIENT_ID` / `SECRET` — Google OAuth client ID
- `GITHUB_CLIENT_ID` / `SECRET` — GitHub **OAuth login** (identity only; not App install)
- `GLUU_ISSUER` — Gluu server issuer URL (must match `/.well-known/openid-configuration`)
- `GLUU_CLIENT_ID` - Gluu OAuth client ID
- `GLUU_CLIENT_SECRET` - Gluu OAuth client secret
- `NEXT_PUBLIC_GLUU_ENABLED` - Set to `true` to show the Gluu sign-in button

### Gluu Setup (optional)

1. In Gluu oxTrust, create an OpenID Connect client.
2. Set the redirect URI to `{NEXTAUTH_URL}/api/auth/callback/gluu` (e.g. `http://localhost:3000/api/auth/callback/gluu` for local dev).
3. Enable scopes: `openid`, `email`, `profile`.
4. Copy the client ID and secret into `.env`.
5. Set `GLUU_ISSUER` to the exact `issuer` value from `https://{hostname}/.well-known/openid-configuration`.
6. Set `NEXT_PUBLIC_GLUU_ENABLED=true` to show the Gluu button on the login page.

### Frontend
- `NEXT_PUBLIC_API_URL` - Backend API URL
- `NEXTAUTH_URL` - Application URL
- `NEXTAUTH_SECRET` - NextAuth secret

### Worker
- `CHECKMATE_PATH` - Path to go-checkmate binary

## Deployment

See [DEPLOYMENT.md](./DEPLOYMENT.md) for detailed deployment guides:

- **AWS ECS Fargate** - Scalable container deployment
- **Google Cloud Run** - Serverless container deployment
- **Azure Container Apps** - Managed container deployment

## Database Schema

### Users
- OAuth2 authentication (Google, GitHub, Gluu)
- Role-based access control
- Profile information

### Projects
- Repository information
- Language detection
- Created by user

### Scans
- Scan status tracking
- Tool configuration
- Results summary

### Findings
- Vulnerability details
- Severity classification
- False positive management

### Schedules
- Cron-based scheduling
- Tool configuration
- Enabled/disabled state

## Security Considerations

1. **Authentication**: OAuth2 with secure token storage
2. **Authorization**: Role-based access control
3. **Data Encryption**: TLS for all communications
4. **Secrets Management**: Use environment variables or secret managers
5. **SQL Injection**: Parameterized queries via GORM
6. **XSS Prevention**: React's built-in escaping
7. **CSRF Protection**: NextAuth.js session management

## Monitoring

### Health Checks
- Backend: `GET /api/v1/health`
- Database: Connection pool monitoring
- Redis: Connection monitoring

### Logging
- Structured logging in Go
- Console logging in Next.js
- Cloud provider integration

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

For issues and questions:
- Codeberg Issues: https://codeberg.org/marcinguy/checkmate-go/issues
- Documentation: [DEPLOYMENT.md](./DEPLOYMENT.md)
