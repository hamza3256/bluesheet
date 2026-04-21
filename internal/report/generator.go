package report

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"time"

	"github.com/hamza3256/bluesheet/internal/domain"
)

// Generator produces trade report data for a given request.
type Generator interface {
	Generate(ctx context.Context, req *domain.BlueSheetRequest, w io.Writer) (rowCount int64, err error)
}

// StubGenerator produces synthetic trade rows for testing / demonstration.
type StubGenerator struct{}

func NewStubGenerator() *StubGenerator {
	return &StubGenerator{}
}

func (g *StubGenerator) Generate(ctx context.Context, req *domain.BlueSheetRequest, w io.Writer) (int64, error) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"trade_id", "ticker", "side", "quantity", "price", "executed_at", "account_id"}
	if err := cw.Write(header); err != nil {
		return 0, fmt.Errorf("write header: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numRows := rng.Intn(50) + 10
	sleepPerRow := time.Duration(rng.Intn(50)+10) * time.Millisecond

	var count int64
	for i := 0; i < numRows; i++ {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}

		time.Sleep(sleepPerRow)

		execTime := req.StartTime.Add(time.Duration(rng.Int63n(int64(req.EndTime.Sub(req.StartTime)))))
		side := "buy"
		if rng.Intn(2) == 0 {
			side = "sell"
		}

		row := []string{
			fmt.Sprintf("T-%s-%06d", req.Ticker, i+1),
			req.Ticker,
			side,
			strconv.Itoa(rng.Intn(1000) + 1),
			fmt.Sprintf("%.2f", 100+rng.Float64()*200),
			execTime.Format(time.RFC3339),
			fmt.Sprintf("ACCT-%04d", rng.Intn(500)+1),
		}
		if err := cw.Write(row); err != nil {
			return count, fmt.Errorf("write row %d: %w", i, err)
		}
		count++
	}
	return count, nil
}
