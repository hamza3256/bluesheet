package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	StatusQueued    RequestStatus = "queued"
	StatusRunning   RequestStatus = "running"
	StatusSucceeded RequestStatus = "succeeded"
	StatusFailed    RequestStatus = "failed"
)

type BlueSheetRequest struct {
	ID           uuid.UUID     `json:"id"`
	Ticker       string        `json:"ticker"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Status       RequestStatus `json:"status"`
	CallbackURL  *string       `json:"callback_url,omitempty"`
	ErrorMessage *string       `json:"error_message,omitempty"`
	S3Key        *string       `json:"s3_key,omitempty"`
	ETag         *string       `json:"etag,omitempty"`
	RowCount     *int64        `json:"row_count,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type CreateRequestInput struct {
	Ticker      string    `json:"ticker"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	CallbackURL *string   `json:"callback_url,omitempty"`
}

func (in *CreateRequestInput) Validate() error {
	in.Ticker = strings.TrimSpace(strings.ToUpper(in.Ticker))
	if in.Ticker == "" {
		return fmt.Errorf("ticker is required")
	}
	if in.StartTime.IsZero() || in.EndTime.IsZero() {
		return fmt.Errorf("start_time and end_time are required")
	}
	if !in.EndTime.After(in.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	return nil
}

// TradeRow represents a single trade in the report.
type TradeRow struct {
	TradeID    string
	Ticker     string
	Side       string // "buy" or "sell"
	Quantity   int64
	Price      float64
	ExecutedAt time.Time
	AccountID  string
}
