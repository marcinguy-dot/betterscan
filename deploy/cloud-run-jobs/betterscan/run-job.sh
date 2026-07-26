#!/usr/bin/env bash
set -euo pipefail

# Usage:
# ./run-job.sh <gcp-project> <region> <image> <tasks> [parallelism]
# Example:
# ./run-job.sh my-project europe-west1 europe-west1-docker.pkg.dev/my-project/sec/betterscan:latest 500 100

PROJECT="${1:?gcp project is required}"
REGION="${2:?region is required}"
IMAGE="${3:?image is required}"
TASKS="${4:?task count is required}"
PARALLELISM="${5:-$TASKS}"

JOB_NAME="${JOB_NAME:-betterscan-scan}"

gcloud config set project "${PROJECT}"

gcloud run jobs deploy "${JOB_NAME}" \
  --image "${IMAGE}" \
  --region "${REGION}" \
  --tasks "${TASKS}" \
  --parallelism "${PARALLELISM}" \
  --max-retries 1 \
  --task-timeout 3600 \
  --args="--strategy=parallel,--json-out=/tmp/results.json,--sarif-out=/tmp/results.sarif,--html-out=/tmp/results.html"

gcloud run jobs execute "${JOB_NAME}" --region "${REGION}" --wait
