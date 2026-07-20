# Checkmate Web Platform

A modern web interface for the Checkmate code scanning platform with SSO authentication, dashboard, and vulnerability management.

## Architecture

```
checkmate-web/
├── frontend/          # Next.js 14 + TypeScript + shadcn/ui
├── backend/           # Go API server with Gin
├── worker/            # Go scan worker service
├── docker-compose.yml # Local development
├── DEPLOYMENT.md      # Deployment guide
└── .env.example       # Environment variables template
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
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379

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

### Authentication
- `GET /api/v1/auth/login` - Initiate OAuth login
- `GET /api/v1/auth/callback` - OAuth callback
- `GET /api/v1/auth/logout` - Logout

### Projects
- `GET /api/v1/projects` - List all projects
- `POST /api/v1/projects` - Create project
- `GET /api/v1/projects/:id` - Get project details
- `PUT /api/v1/projects/:id` - Update project
- `DELETE /api/v1/projects/:id` - Delete project

### Scans
- `GET /api/v1/scans` - List scans (filter by project_id)
- `POST /api/v1/scans` - Create new scan
- `GET /api/v1/scans/:id` - Get scan details
- `GET /api/v1/scans/:id/findings` - Get scan findings

### Findings
- `GET /api/v1/findings` - List findings (filter by severity, project)
- `PUT /api/v1/findings/:id/false-positive` - Mark as false positive

### Schedules
- `GET /api/v1/schedules` - List schedules
- `POST /api/v1/schedules` - Create schedule
- `PUT /api/v1/schedules/:id` - Update schedule
- `DELETE /api/v1/schedules/:id` - Delete schedule

### Dashboard
- `GET /api/v1/dashboard/stats` - Get dashboard statistics
- `GET /api/v1/dashboard/trends` - Get vulnerability trends

## Environment Variables

See `.env.example` for required environment variables:

### Database
- `DATABASE_URL` - PostgreSQL connection string

### Redis
- `REDIS_ADDR` - Redis server address
- `REDIS_PASSWORD` - Redis password (optional)

### Backend
- `PORT` - Backend port (default: 8080)
- `JWT_SECRET` - JWT signing secret

### OAuth2
- `GOOGLE_CLIENT_ID` - Google OAuth client ID
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret
- `GITHUB_CLIENT_ID` - GitHub OAuth client ID
- `GITHUB_CLIENT_SECRET` - GitHub OAuth client secret
- `GLUU_ISSUER` - Gluu server issuer URL (must match `/.well-known/openid-configuration`)
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
