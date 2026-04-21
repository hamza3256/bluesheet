# Blue Sheet Regulatory Reporting System

Async job-processing service that accepts regulator blue-sheet requests (ticker + date range), generates a CSV trade report, and uploads it to S3-compatible storage.

## Architecture

```
POST /v1/report-requests  →  PostgreSQL (queued)  →  Worker (SKIP LOCKED)
                                                        ├─ Generate CSV (stub)
                                                        ├─ Upload to S3
                                                        └─ Mark succeeded
```

Single binary with subcommands: `api`, `worker`, `migrate`.

## Quick start

```bash
# Start Postgres + LocalStack (S3)
make docker-up

# Run database migrations
make migrate

# Start the API server (default :8080)
make run-api

# In a separate terminal, start the worker
make run-worker
```

### Example requests

```bash
# Create a report request
curl -s -X POST http://localhost:8080/v1/report-requests \
  -H 'Content-Type: application/json' \
  -d '{"ticker":"AAPL","start_time":"2023-11-01T00:00:00Z","end_time":"2023-12-01T00:00:00Z"}' | jq .

# Check status (replace <id> with the returned id)
curl -s http://localhost:8080/v1/report-requests/<id> | jq .
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://bluesheet:bluesheet@localhost:5432/bluesheet?sslmode=disable` | PostgreSQL connection string |
| `S3_ENDPOINT` | `http://localhost:4566` | S3-compatible endpoint (LocalStack) |
| `S3_BUCKET` | `bluesheets` | Target bucket for report uploads |
| `S3_REGION` | `us-east-1` | AWS region |
| `AWS_ACCESS_KEY_ID` | `test` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | `test` | AWS secret key |
| `HTTP_ADDR` | `:8080` | API listen address |
| `WORKER_POLL_INTERVAL` | `2s` | How often the worker polls for new jobs |
| `WORKER_CONCURRENCY` | `2` | Number of concurrent worker goroutines |

## Testing

```bash
# Unit tests (no Docker needed)
go test ./internal/domain/...

# Full integration tests (requires Docker for testcontainers)
make test
```

## Project layout

```
cmd/bluesheet/       Single binary entrypoint (api | worker | migrate)
internal/
  config/            Environment-based configuration
  domain/            Types, validation
  httpapi/           REST handlers (create, get)
  report/            CSV generator interface + stub
  storage/           S3 upload abstraction
  store/             PostgreSQL repository + migrations
  worker/            Poll loop, orchestration (generate → upload → done)
migrations/          SQL migration files (also embedded in store package)
```
