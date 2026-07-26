# AWS ECS/Fargate deployment

This folder contains minimal ECS/Fargate templates for running `betterscan` N times in parallel.

## Register task definition

```bash
aws ecs register-task-definition \
  --cli-input-json file://deploy/ecs-fargate/betterscan/task-definition.json
```

## Run N tasks

```bash
cd deploy/ecs-fargate/betterscan
./run-tasks.sh my-cluster betterscan-scan "subnet-aaa,subnet-bbb" "sg-xxx" 500
```

- `count` is the number of Fargate tasks to run.
- Adjust CPU/memory and image in `task-definition.json` before registration.
