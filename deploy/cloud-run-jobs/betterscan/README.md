# Cloud Run Jobs deployment

This folder contains a minimal template to run `betterscan` as Google Cloud Run Jobs with N tasks.

## Build and push image

```bash
gcloud builds submit --tag REGION-docker.pkg.dev/PROJECT/REPO/betterscan:latest .
```

## Run N tasks

```bash
cd deploy/cloud-run-jobs/betterscan
./run-job.sh PROJECT REGION REGION-docker.pkg.dev/PROJECT/REPO/betterscan:latest 500 100
```

- `tasks` is total tasks to run.
- `parallelism` is concurrent running tasks (defaults to `tasks`).
