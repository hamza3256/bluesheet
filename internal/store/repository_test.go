package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hamza3256/bluesheet/internal/domain"
	"github.com/hamza3256/bluesheet/internal/store"
)

func setupTestDB(t *testing.T) *store.Repository {
	t.Helper()
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("bluesheet_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgC.Terminate(ctx); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	})

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := store.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return store.New(pool)
}

func TestCreateAndGetRequest(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	in := domain.CreateRequestInput{
		Ticker:    "AAPL",
		StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	req, err := repo.CreateRequest(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if req.Ticker != "AAPL" {
		t.Errorf("ticker = %s, want AAPL", req.Ticker)
	}
	if req.Status != domain.StatusQueued {
		t.Errorf("status = %s, want queued", req.Status)
	}

	got, err := repo.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != req.ID {
		t.Errorf("id mismatch: %s != %s", got.ID, req.ID)
	}
}

func TestDequeueRequest(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	for _, sym := range []string{"AAPL", "GOOG"} {
		in := domain.CreateRequestInput{
			Ticker:    sym,
			StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
		}
		if _, err := repo.CreateRequest(ctx, in); err != nil {
			t.Fatalf("create %s: %v", sym, err)
		}
	}

	first, err := repo.DequeueRequest(ctx)
	if err != nil {
		t.Fatalf("dequeue 1: %v", err)
	}
	if first == nil {
		t.Fatal("dequeue 1 returned nil")
	}
	if first.Ticker != "AAPL" {
		t.Errorf("first dequeue ticker = %s, want AAPL", first.Ticker)
	}
	if first.Status != domain.StatusRunning {
		t.Errorf("first dequeue status = %s, want running", first.Status)
	}

	second, err := repo.DequeueRequest(ctx)
	if err != nil {
		t.Fatalf("dequeue 2: %v", err)
	}
	if second == nil {
		t.Fatal("dequeue 2 returned nil")
	}
	if second.Ticker != "GOOG" {
		t.Errorf("second dequeue ticker = %s, want GOOG", second.Ticker)
	}

	empty, err := repo.DequeueRequest(ctx)
	if err != nil {
		t.Fatalf("dequeue 3: %v", err)
	}
	if empty != nil {
		t.Errorf("expected nil, got %+v", empty)
	}
}

func TestCompleteAndFailRequest(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Create and dequeue a request, then complete it.
	in := domain.CreateRequestInput{
		Ticker:    "MSFT",
		StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
	}
	req, err := repo.CreateRequest(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.DequeueRequest(ctx); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	err = repo.CompleteRequest(ctx, req.ID, "some/key.csv", `"abc"`, 42)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := repo.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.StatusSucceeded {
		t.Errorf("status = %s, want succeeded", got.Status)
	}
	if got.S3Key == nil || *got.S3Key != "some/key.csv" {
		t.Errorf("s3_key = %v, want some/key.csv", got.S3Key)
	}
	if got.RowCount == nil || *got.RowCount != 42 {
		t.Errorf("row_count = %v, want 42", got.RowCount)
	}

	// Create another request and fail it.
	req2, _ := repo.CreateRequest(ctx, in)
	repo.DequeueRequest(ctx)

	err = repo.FailRequest(ctx, req2.ID, "something broke")
	if err != nil {
		t.Fatalf("fail: %v", err)
	}
	got2, _ := repo.GetRequest(ctx, req2.ID)
	if got2.Status != domain.StatusFailed {
		t.Errorf("status = %s, want failed", got2.Status)
	}
	if got2.ErrorMessage == nil || *got2.ErrorMessage != "something broke" {
		t.Errorf("error_message = %v, want 'something broke'", got2.ErrorMessage)
	}
}
