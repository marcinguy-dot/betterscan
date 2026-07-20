package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const scanQueueKey = "scan:queue"

// ScanJob is the payload consumed by the worker (must stay in sync with worker/main.go).
type ScanJob struct {
	ScanID        string `json:"scan_id"`
	ProjectID     string `json:"project_id"`
	RepoURL       string `json:"repo_url"`
	RepoBranch    string `json:"repo_branch"`
	Tools         string `json:"tools"`
	Strategy      string `json:"strategy"`
	CloneUsername string `json:"clone_username,omitempty"`
	ClonePassword string `json:"clone_password,omitempty"`
}

type queueService struct {
	rdb *redis.Client
}

func newQueueService() (*queueService, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	return &queueService{rdb: rdb}, nil
}

func (q *queueService) EnqueueScan(job ScanJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return q.rdb.LPush(ctx, scanQueueKey, data).Err()
}
