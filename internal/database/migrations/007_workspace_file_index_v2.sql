ALTER TABLE file_index ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE file_index ADD COLUMN dir TEXT NOT NULL DEFAULT '';
ALTER TABLE file_index ADD COLUMN extension TEXT NOT NULL DEFAULT '';
ALTER TABLE file_index ADD COLUMN kind TEXT NOT NULL DEFAULT 'file';
ALTER TABLE file_index ADD COLUMN index_status TEXT NOT NULL DEFAULT 'indexed';
ALTER TABLE file_index ADD COLUMN path_lower TEXT NOT NULL DEFAULT '';
ALTER TABLE file_index ADD COLUMN name_lower TEXT NOT NULL DEFAULT '';
ALTER TABLE file_index ADD COLUMN depth INTEGER NOT NULL DEFAULT 0;

UPDATE file_index
SET
  name = path,
  dir = '',
  extension = '',
  path_lower = lower(path),
  name_lower = lower(path),
  depth = length(path) - length(replace(path, '/', '')) + 1;

CREATE INDEX IF NOT EXISTS idx_file_index_workspace_kind_path ON file_index(workspace_id, kind, path);
CREATE INDEX IF NOT EXISTS idx_file_index_workspace_name_lower ON file_index(workspace_id, name_lower);
CREATE INDEX IF NOT EXISTS idx_file_index_workspace_path_lower ON file_index(workspace_id, path_lower);
CREATE INDEX IF NOT EXISTS idx_file_index_workspace_dir ON file_index(workspace_id, dir);

CREATE TABLE IF NOT EXISTS workspace_index_state (
  workspace_id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  last_indexed_at datetime,
  last_full_scan_at datetime,
  file_count INTEGER NOT NULL,
  skipped_count INTEGER NOT NULL,
  truncated numeric NOT NULL,
  error TEXT,
  updated_at datetime NOT NULL
);
