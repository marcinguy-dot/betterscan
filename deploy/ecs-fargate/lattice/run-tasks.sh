#!/usr/bin/env bash
set -euo pipefail

# Usage:
# ./run-tasks.sh <cluster> <task-def> <subnets> <security-groups> <count>
# Example:
# ./run-tasks.sh prod-cluster lattice-scan "subnet-aaa,subnet-bbb" "sg-xxx" 500

CLUSTER="${1:?cluster is required}"
TASK_DEF="${2:?task definition family or arn is required}"
SUBNETS="${3:?comma-separated subnets required}"
SECURITY_GROUPS="${4:?comma-separated security groups required}"
COUNT="${5:?task count is required}"

aws ecs run-task \
  --cluster "${CLUSTER}" \
  --launch-type FARGATE \
  --task-definition "${TASK_DEF}" \
  --count "${COUNT}" \
  --network-configuration "awsvpcConfiguration={subnets=[${SUBNETS}],securityGroups=[${SECURITY_GROUPS}],assignPublicIp=ENABLED}"
