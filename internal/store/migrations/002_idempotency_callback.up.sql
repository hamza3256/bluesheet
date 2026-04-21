-- Idempotency: same (ticker, start_time, end_time) returns existing request.
CREATE UNIQUE INDEX idx_requests_dedupe ON blue_sheet_requests (ticker, start_time, end_time);

-- Optional webhook URL the worker will POST to when the job reaches a terminal state.
ALTER TABLE blue_sheet_requests ADD COLUMN callback_url TEXT;
