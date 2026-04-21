package domain

import (
	"testing"
	"time"
)

func TestCreateRequestInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateRequestInput
		wantErr bool
	}{
		{
			name: "valid",
			input: CreateRequestInput{
				Ticker:    "aapl",
				StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "normalises ticker to upper",
			input: CreateRequestInput{
				Ticker:    "  aapl  ",
				StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "empty ticker",
			input: CreateRequestInput{
				StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			},
			wantErr: true,
		},
		{
			name: "end before start",
			input: CreateRequestInput{
				Ticker:    "AAPL",
				StartTime: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
			},
			wantErr: true,
		},
		{
			name: "same start and end",
			input: CreateRequestInput{
				Ticker:    "AAPL",
				StartTime: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && tc.input.Ticker != "AAPL" {
				t.Errorf("ticker not normalised: %s", tc.input.Ticker)
			}
		})
	}
}
