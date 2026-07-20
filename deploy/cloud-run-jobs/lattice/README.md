# Cloud Run Jobs deployment

This folder contains a minimal template to run `lattice` as Google Cloud Run Jobs with N tasks.

## Build and push image

```bash
gcloud builds submit --tag REGION-docker.pkg.dev/PROJECT/REPO/lattice:latest .
```

## Run N tasks

```bash
cd deploy/cloud-run-jobs/lattice
./run-job.sh PROJECT REGION REGION-docker.pkg.dev/PROJECT/REPO/lattice:latest 500 100
```

- `tasks` is total tasks to run.
- `parallelism` is concurrent running tasks (defaults to `tasks`).
