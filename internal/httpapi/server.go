package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/hamza3256/bluesheet/internal/config"
	"github.com/hamza3256/bluesheet/internal/domain"
	"github.com/hamza3256/bluesheet/internal/storage"
	"github.com/hamza3256/bluesheet/internal/store"
)

type Server struct {
	cfg       *config.Config
	repo      *store.Repository
	presigner storage.Presigner
	mux       *http.ServeMux
}

// presigner may be nil (no download_url on GET until configured).
func NewServer(cfg *config.Config, repo *store.Repository, presigner storage.Presigner) *Server {
	s := &Server{cfg: cfg, repo: repo, presigner: presigner, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/report-requests", s.handleCreate)
	s.mux.HandleFunc("GET /v1/report-requests/{id}", s.handleGet)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.HTTPAddr,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateRequestInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := in.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req, err := s.repo.CreateRequest(r.Context(), in)
	if err != nil {
		slog.Error("create request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, s.reportJSON(r.Context(), req))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	req, err := s.repo.GetRequest(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, s.reportJSON(r.Context(), req))
}

// reportJSON attaches s3_url and optional presigned download_url when the job succeeded.
type reportResponse struct {
	*domain.BlueSheetRequest
	S3URL       *string `json:"s3_url,omitempty"`
	DownloadURL *string `json:"download_url,omitempty"`
}

func (s *Server) reportJSON(ctx context.Context, req *domain.BlueSheetRequest) reportResponse {
	resp := reportResponse{BlueSheetRequest: req}
	if req.Status == domain.StatusSucceeded && req.S3Key != nil && *req.S3Key != "" {
		u := fmt.Sprintf("s3://%s/%s", s.cfg.S3Bucket, *req.S3Key)
		resp.S3URL = &u
		if s.presigner != nil {
			url, err := s.presigner.PresignedGetURL(ctx, s.cfg.S3Bucket, *req.S3Key, s.cfg.PresignGetURLDuration)
			if err != nil {
				slog.Warn("presigned download URL failed", "request_id", req.ID, "error", err)
			} else {
				resp.DownloadURL = &url
			}
		}
	}
	return resp
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
