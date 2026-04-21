package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hamza3256/bluesheet/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateRequest(ctx context.Context, in domain.CreateRequestInput) (*domain.BlueSheetRequest, error) {
	req := domain.BlueSheetRequest{
		ID:        uuid.New(),
		Ticker:    in.Ticker,
		StartTime: in.StartTime,
		EndTime:   in.EndTime,
		Status:    domain.StatusQueued,
	}

	const q = `
		INSERT INTO blue_sheet_requests (id, ticker, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	err := r.pool.QueryRow(ctx, q,
		req.ID, req.Ticker, req.StartTime, req.EndTime, req.Status,
	).Scan(&req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert request: %w", err)
	}
	return &req, nil
}

func (r *Repository) GetRequest(ctx context.Context, id uuid.UUID) (*domain.BlueSheetRequest, error) {
	const q = `
		SELECT id, ticker, start_time, end_time, status,
		       error_message, s3_key, etag, row_count, created_at, updated_at
		FROM blue_sheet_requests WHERE id = $1`

	var req domain.BlueSheetRequest
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&req.ID, &req.Ticker, &req.StartTime, &req.EndTime, &req.Status,
		&req.ErrorMessage, &req.S3Key, &req.ETag, &req.RowCount, &req.CreatedAt, &req.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("request %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return &req, nil
}

// DequeueRequest atomically claims the next queued job using SKIP LOCKED,
// transitioning it to "running". Returns nil if no work is available.
func (r *Repository) DequeueRequest(ctx context.Context) (*domain.BlueSheetRequest, error) {
	const q = `
		UPDATE blue_sheet_requests
		SET status = 'running', updated_at = now()
		WHERE id = (
			SELECT id FROM blue_sheet_requests
			WHERE status = 'queued'
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, ticker, start_time, end_time, status,
		          error_message, s3_key, etag, row_count, created_at, updated_at`

	var req domain.BlueSheetRequest
	err := r.pool.QueryRow(ctx, q).Scan(
		&req.ID, &req.Ticker, &req.StartTime, &req.EndTime, &req.Status,
		&req.ErrorMessage, &req.S3Key, &req.ETag, &req.RowCount, &req.CreatedAt, &req.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue: %w", err)
	}
	return &req, nil
}

// CompleteRequest marks a request as succeeded and records the upload metadata.
func (r *Repository) CompleteRequest(ctx context.Context, id uuid.UUID, s3Key, etag string, rowCount int64) error {
	const q = `
		UPDATE blue_sheet_requests
		SET status = 'succeeded', s3_key = $2, etag = $3, row_count = $4, updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id, s3Key, etag, rowCount)
	if err != nil {
		return fmt.Errorf("complete request: %w", err)
	}
	return nil
}

// FailRequest marks a request as failed with an error message.
func (r *Repository) FailRequest(ctx context.Context, id uuid.UUID, errMsg string) error {
	const q = `
		UPDATE blue_sheet_requests
		SET status = 'failed', error_message = $2, updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id, errMsg)
	if err != nil {
		return fmt.Errorf("fail request: %w", err)
	}
	return nil
}
