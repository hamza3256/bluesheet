package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hamza3256/bluesheet/internal/config"
	"github.com/hamza3256/bluesheet/internal/domain"
	"github.com/hamza3256/bluesheet/internal/httpapi"
	"github.com/hamza3256/bluesheet/internal/store"
)

func setupAPI(t *testing.T) *httptest.Server {
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
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

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

	repo := store.New(pool)
	cfg := &config.Config{HTTPAddr: ":0"}
	srv := httpapi.NewServer(cfg, repo)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestCreateAndGetEndpoints(t *testing.T) {
	ts := setupAPI(t)

	body := map[string]any{
		"ticker":     "AAPL",
		"start_time": "2023-11-01T00:00:00Z",
		"end_time":   "2023-12-01T00:00:00Z",
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(ts.URL+"/v1/report-requests", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var created domain.BlueSheetRequest
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Ticker != "AAPL" {
		t.Errorf("ticker = %s, want AAPL", created.Ticker)
	}

	resp2, err := http.Get(ts.URL + "/v1/report-requests/" + created.ID.String())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp2.StatusCode)
	}
}

func TestValidationErrors(t *testing.T) {
	ts := setupAPI(t)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"missing ticker", map[string]any{"start_time": "2023-11-01T00:00:00Z", "end_time": "2023-12-01T00:00:00Z"}},
		{"end before start", map[string]any{"ticker": "AAPL", "start_time": "2023-12-01T00:00:00Z", "end_time": "2023-11-01T00:00:00Z"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			resp, err := http.Post(ts.URL+"/v1/report-requests", "application/json", bytes.NewReader(b))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}
