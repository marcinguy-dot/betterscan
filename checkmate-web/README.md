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

- **Authentication**: SSO via OAuth2/OIDC (Google, GitHub)
- **Dashboard**: Scan history, vulnerability trends, statistics
- **Project Management**: Add/remove projects, configure rules
- **Scan Execution**: Trigger scans with configurable tools and strategies
- **Vulnerability Tracking**: Track findings by severity (Critical, High, Medium, Low)
- **False Positive Management**: Mark and suppress findings
- **Real-time Updates**: WebSocket support for scan progress
- **API Integration**: RESTful API for automation
- **Multi-cloud Deployment**: Support for AWS ECS, Google Cloud Run, Azure Container Apps

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
git clone <repository-url>
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
- OAuth2 authentication (Google, GitHub)
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
- GitHub Issues: <repository-url>/issues
- Documentation: [DEPLOYMENT.md](./DEPLOYMENT.md)
