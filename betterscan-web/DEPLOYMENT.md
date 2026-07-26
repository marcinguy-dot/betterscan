# Deployment Guide

This guide covers deploying BetterScan Web to production using various platforms.

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
docker build -t betterscan-backend ./backend
docker tag betterscan-backend:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-backend:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-backend:latest

# Build worker
docker build -t betterscan-worker ./worker
docker tag betterscan-worker:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-worker:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-worker:latest

# Build frontend
docker build -t betterscan-frontend ./frontend
docker tag betterscan-frontend:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-frontend:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/betterscan-frontend:latest
```

2. **Create ECS task definitions**

Use the CloudFormation templates in `deploy/ecs-fargate/`:

```bash
aws cloudformation create-stack \
  --stack-name betterscan-ecs \
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
aws ecs update-service --cluster betterscan --service betterscan-backend

# Deploy worker
aws ecs update-service --cluster betterscan --service betterscan-worker

# Deploy frontend
aws ecs update-service --cluster betterscan --service betterscan-frontend
```

### Scaling

Auto-scaling can be configured via CloudFormation:

```yaml
AutoScalingPolicy:
  Type: AWS::ApplicationAutoScaling::ScalableTarget
  Properties:
    MaxCapacity: 10
    MinCapacity: 2
    ResourceId: service/betterscan/betterscan-backend
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
gcloud run deploy betterscan-backend \
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
gcloud run deploy betterscan-worker \
  --source ./worker \
  --platform managed \
  --region us-central1 \
  --set-env-vars DATABASE_URL=<your-cloudsql-connection-string> \
  --set-env-vars REDIS_ADDR=<your-redis-addr> \
  --set-env-vars BETTERSCAN_PATH=/app/betterscan/betterscan
```

4. **Build and deploy frontend**

```bash
gcloud run deploy betterscan-frontend \
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
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/betterscan-backend', './backend']
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/betterscan-backend']
  - name: 'gcr.io/cloud-builders/gcloud'
    args: ['run', 'deploy', 'betterscan-backend', '--image', 'gcr.io/$PROJECT_ID/betterscan-backend']
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
  --name betterscan-env \
  --resource-group betterscan-rg \
  --location eastus
```

2. **Deploy backend**

```bash
az containerapp create \
  --name betterscan-backend \
  --resource-group betterscan-rg \
  --environment betterscan-env \
  --image <your-registry>/betterscan-backend:latest \
  --env-vars DATABASE_URL=<your-db-url> REDIS_ADDR=<your-redis-url> \
  --target-port 8080 \
  --ingress external
```

3. **Deploy worker**

```bash
az containerapp create \
  --name betterscan-worker \
  --resource-group betterscan-rg \
  --environment betterscan-env \
  --image <your-registry>/betterscan-worker:latest \
  --env-vars DATABASE_URL=<your-db-url> REDIS_ADDR=<your-redis-url> \
  --cpu 2 \
  --memory 4Gi
```

4. **Deploy frontend**

```bash
az containerapp create \
  --name betterscan-frontend \
  --resource-group betterscan-rg \
  --environment betterscan-env \
  --image <your-registry>/betterscan-frontend:latest \
  --env-vars NEXT_PUBLIC_API_URL=<backend-url> \
  --target-port 3000 \
  --ingress external
```

## Monitoring and Logging

### AWS CloudWatch

```bash
# View logs
aws logs tail /ecs/betterscan-backend --follow

# Create metric filters
aws logs put-metric-filter \
  --log-group-name /ecs/betterscan-backend \
  --filter-name ErrorCount \
  --filter-pattern "[timestamp, request_id, level=ERROR, ...]"
```

### Google Cloud Logging

```bash
# View logs
gcloud logging tail "resource.type=cloud_run_revision"

# Create log sinks
gcloud logging sinks create betterscan-errors \
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
  --db-instance-identifier betterscan-db \
  --db-snapshot-identifier betterscan-snapshot-$(date +%Y%m%d)

# Google Cloud SQL
gcloud sql backups create \
  --instance=betterscan-db \
  --description="Daily backup"

# Azure Database
az postgres db create \
  --name betterscan-db \
  --resource-group betterscan-rg \
  --server-name betterscan-server \
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
- Verify BETTERSCAN_PATH is correct
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
