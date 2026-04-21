package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL string

	S3Endpoint  string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string

	HTTPAddr string

	// how long presigned S3 download links stay valid
	PresignGetURLDuration time.Duration

	WorkerPollInterval time.Duration
	WorkerConcurrency  int
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:           envOr("DATABASE_URL", "postgres://bluesheet:bluesheet@localhost:5432/bluesheet?sslmode=disable"),
		S3Endpoint:            envOr("S3_ENDPOINT", "http://localhost:4566"),
		S3Bucket:              envOr("S3_BUCKET", "bluesheets"),
		S3Region:              envOr("S3_REGION", "us-east-1"),
		S3AccessKey:           envOr("AWS_ACCESS_KEY_ID", "test"),
		S3SecretKey:           envOr("AWS_SECRET_ACCESS_KEY", "test"),
		HTTPAddr:              envOr("HTTP_ADDR", ":8080"),
		PresignGetURLDuration: 1 * time.Hour,
		WorkerPollInterval:    2 * time.Second,
		WorkerConcurrency:     2,
	}

	if v := os.Getenv("WORKER_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WORKER_POLL_INTERVAL: %w", err)
		}
		c.WorkerPollInterval = d
	}
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid WORKER_CONCURRENCY: %s", v)
		}
		c.WorkerConcurrency = n
	}
	if v := os.Getenv("PRESIGN_GET_URL_DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PRESIGN_GET_URL_DURATION: %w", err)
		}
		c.PresignGetURLDuration = d
	}

	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
