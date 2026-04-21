ALTER TABLE blue_sheet_requests DROP COLUMN IF EXISTS callback_url;
DROP INDEX IF EXISTS idx_requests_dedupe;
