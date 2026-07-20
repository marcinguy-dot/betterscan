# Deployment Guide

This guide covers deploying Lattice Web to production using various platforms.

## Prerequisites

- Docker and Docker Compose installed
- Cloud provider account (AWS, GCP, or Azure)
- Domain name (optional, for custom URLs)
- OAuth2 credentials (Google/GitHub/Auth0)

## Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Required variables:
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_ADDR`: Redis server address
- `JWT_SECRET`: Secret for JWT tokens
- `NEXTAUTH_SECRET`: Secret for NextAuth.js
- OAuth2 client IDs and secrets

## Local Development

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

Access:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- PostgreSQL: localhost:5432
- Redis: localhost:6379

## AWS ECS Fargate Deployment

### Prerequisites

- AWS CLI configured
- ECS cluster created
- RDS PostgreSQL instance created
- ElastiCache Redis instance created (optional)

### Deployment Steps

1. **Build and push images to ECR**

```bash
# Login to ECR
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <account-id>.dkr.ecr.us-east-1.amazonaws.com

# Build backend
docker build -t lattice-backend ./backend
docker tag lattice-backend:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-backend:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-backend:latest

# Build worker
docker build -t lattice-worker ./worker
docker tag lattice-worker:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-worker:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-worker:latest

# Build frontend
docker build -t lattice-frontend ./frontend
docker tag lattice-frontend:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-frontend:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/lattice-frontend:latest
```

2. **Create ECS task definitions**

Use the CloudFormation templates in `deploy/ecs-fargate/`:

```bash
aws cloudformation create-stack \
  --stack-name lattice-ecs \
  --template-body file://deploy/ecs-fargate/template.yaml \
  --parameters \
    ParameterKey=DatabaseURL,ParameterValue=<your-rds-url> \
    ParameterKey=RedisAddr,ParameterValue=<your-redis-url> \
    ParameterKey=JwtSecret,ParameterValue=<your-jwt-secret> \
  --capabilities CAPABILITY_IAM
```

3. **Deploy services**

```bash
# Deploy backend
aws ecs update-service --cluster lattice --service lattice-backend

# Deploy worker
aws ecs update-service --cluster lattice --service lattice-worker

# Deploy frontend
aws ecs update-service --cluster lattice --service lattice-frontend
```

### Scaling

Auto-scaling can be configured via CloudFormation:

```yaml
AutoScalingPolicy:
  Type: AWS::ApplicationAutoScaling::ScalableTarget
  Properties:
    MaxCapacity: 10
    MinCapacity: 2
    ResourceId: service/lattice/lattice-backend
    ScalableDimension: ecs:service:DesiredCount
    ServiceNamespace: ecs
```

## Google Cloud Run Deployment

### Prerequisites

- gcloud CLI configured
- Cloud SQL PostgreSQL instance created
- Memorystore Redis instance created (optional)

### Deployment Steps

1. **Enable required APIs**

```bash
gcloud services enable \
  run.googleapis.com \
  sqladmin.googleapis.com \
  redis.googleapis.com \
  cloudbuild.googleapis.com
```

2. **Build and deploy backend**

```bash
gcloud run deploy lattice-backend \
  --source ./backend \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars DATABASE_URL=<your-cloudsql-connection-string> \
  --set-env-vars REDIS_ADDR=<your-redis-addr> \
  --set-env-vars JWT_SECRET=<your-secret> \
  --set-env-vars GOOGLE_CLIENT_ID=<your-client-id> \
  --set-env-vars GOOGLE_CLIENT_SECRET=<your-client-secret>
```

3. **Build and deploy worker**

```bash
gcloud run deploy lattice-worker \
  --source ./worker \
  --platform managed \
  --region us-central1 \
  --set-env-vars DATABASE_URL=<your-cloudsql-connection-string> \
  --set-env-vars REDIS_ADDR=<your-redis-addr> \
  --set-env-vars LATTICE_PATH=/app/lattice/lattice
```

4. **Build and deploy frontend**

```bash
gcloud run deploy lattice-frontend \
  --source ./frontend \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars NEXT_PUBLIC_API_URL=<backend-url> \
  --set-env-vars NEXTAUTH_URL=<your-domain> \
  --set-env-vars NEXTAUTH_SECRET=<your-secret>
```

### Cloud Build Automation

Create `cloudbuild.yaml`:

```yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/lattice-backend', './backend']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/lattice-backend']
  - name: 'gcr.io/cloud-builders/gcloud'
    args: ['run', 'deploy', 'lattice-backend', '--image', 'gcr.io/$PROJECT_ID/lattice-backend']
```

## Azure Container Apps Deployment

### Prerequisites

- Azure CLI configured
- Azure Container Apps environment created
- Azure Database for PostgreSQL created
- Azure Cache for Redis created (optional)

### Deployment Steps

1. **Create Container App environment**

```bash
az containerapp env create \
  --name lattice-env \
  --resource-group lattice-rg \
  --location eastus
```

2. **Deploy backend**

```bash
az containerapp create \
  --name lattice-backend \
  --resource-group lattice-rg \
  --environment lattice-env \
  --image <your-registry>/lattice-backend:latest \
  --env-vars DATABASE_URL=<your-db-url> REDIS_ADDR=<your-redis-url> \
  --target-port 8080 \
  --ingress external
```

3. **Deploy worker**

```bash
az containerapp create \
  --name lattice-worker \
  --resource-group lattice-rg \
  --environment lattice-env \
  --image <your-registry>/lattice-worker:latest \
  --env-vars DATABASE_URL=<your-db-url> REDIS_ADDR=<your-redis-url> \
  --cpu 2 \
  --memory 4Gi
```

4. **Deploy frontend**

```bash
az containerapp create \
  --name lattice-frontend \
  --resource-group lattice-rg \
  --environment lattice-env \
  --image <your-registry>/lattice-frontend:latest \
  --env-vars NEXT_PUBLIC_API_URL=<backend-url> \
  --target-port 3000 \
  --ingress external
```

## Monitoring and Logging

### AWS CloudWatch

```bash
# View logs
aws logs tail /ecs/lattice-backend --follow

# Create metric filters
aws logs put-metric-filter \
  --log-group-name /ecs/lattice-backend \
  --filter-name ErrorCount \
  --filter-pattern "[timestamp, request_id, level=ERROR, ...]"
```

### Google Cloud Logging

```bash
# View logs
gcloud logging tail "resource.type=cloud_run_revision"

# Create log sinks
gcloud logging sinks create lattice-errors \
  bigquery.googleapis.com/projects/<project>/datasets/logs \
  --log-filter='severity>=ERROR'
```

### Azure Monitor

```bash
# View logs
az monitor app-insights query \
  --app <app-insights-id> \
  --analytics-query 'union traces, exceptions | order by timestamp desc'
```

## Security Considerations

1. **Secrets Management**
   - Use AWS Secrets Manager, Google Secret Manager, or Azure Key Vault
   - Never commit secrets to git
   - Rotate secrets regularly

2. **Network Security**
   - Use VPC peering or private endpoints
   - Enable TLS/SSL everywhere
   - Implement WAF rules

3. **Authentication**
   - Enable OAuth2 with proper scopes
   - Implement rate limiting
   - Use short-lived tokens

4. **Database Security**
   - Enable encryption at rest
   - Use read replicas for scaling
   - Implement connection pooling

## Backup and Recovery

### PostgreSQL Backups

```bash
# AWS RDS
aws rds create-db-snapshot \
  --db-instance-identifier lattice-db \
  --db-snapshot-identifier lattice-snapshot-$(date +%Y%m%d)

# Google Cloud SQL
gcloud sql backups create \
  --instance=lattice-db \
  --description="Daily backup"

# Azure Database
az postgres db create \
  --name lattice-db \
  --resource-group lattice-rg \
  --server-name lattice-server \
  --geo-redundant-backup Enabled
```

### Disaster Recovery

- Configure multi-region deployment
- Implement automated failover
- Regularly test restore procedures
- Document recovery procedures

## Performance Optimization

1. **Database**
   - Add indexes on frequently queried columns
   - Enable query caching
   - Use connection pooling (PgBouncer)

2. **Redis**
   - Configure persistence (AOF/RDB)
   - Use cluster mode for high availability
   - Monitor memory usage

3. **Application**
   - Enable CDN for static assets
   - Implement response caching
   - Use horizontal pod autoscaling

## Troubleshooting

### Common Issues

**Backend fails to start**
- Check database connection string
- Verify Redis connectivity
- Review logs: `docker-compose logs backend`

**Worker not processing jobs**
- Check Redis queue: `redis-cli LLEN scan:queue`
- Verify LATTICE_PATH is correct
- Review worker logs

**Frontend build errors**
- Clear node_modules: `rm -rf node_modules && npm install`
- Check Next.js version compatibility
- Verify environment variables

### Health Checks

```bash
# Backend health
curl http://localhost:8080/api/v1/health

# Database connection
psql $DATABASE_URL -c "SELECT 1"

# Redis connection
redis-cli ping
```

## Cost Optimization

1. **Use reserved instances** for predictable workloads
2. **Enable auto-scaling** to match demand
3. **Use spot instances** for worker nodes
4. **Implement lifecycle policies** for old data
5. **Monitor and optimize** resource usage
