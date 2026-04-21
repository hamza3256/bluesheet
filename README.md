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
# When `status` is `succeeded`, the response includes a time-limited `download_url`
# (S3 presigned GET) so you can fetch the CSV directly.
curl -s http://localhost:8080/v1/report-requests/<id> | jq .
```

### Idempotency

Requests are deduplicated by **(ticker, start_time, end_time)**. Submitting the same parameters multiple times returns the existing request (with its current status) rather than creating a duplicate.

### Webhook notification (callback_url)

Instead of polling, we can provide a `callback_url` when creating a request. The worker will **POST** to that URL when the job reaches a terminal state (`succeeded` or `failed`).

```bash
curl -s -X POST http://localhost:8080/v1/report-requests \
  -H 'Content-Type: application/json' \
  -d '{
    "ticker":"GOOG",
    "start_time":"2026-01-01T00:00:00Z",
    "end_time":"2026-02-01T00:00:00Z",
    "callback_url":"https://a-service.example/webhook"
  }' | jq .
```

The callback payload looks like:

```json
{"request_id":"<uuid>","ticker":"GOOG","status":"succeeded"}
```

Or on failure:

```json
{"request_id":"<uuid>","ticker":"GOOG","status":"failed","error":"..."}
```

## Testing

```bash
# Unit tests (no Docker needed)
go test ./internal/domain/...

# Full integration tests (requires Docker for testcontainers)
make test
```
