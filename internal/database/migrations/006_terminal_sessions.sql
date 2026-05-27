CREATE TABLE IF NOT EXISTS terminal_sessions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	title TEXT NOT NULL,
	cwd TEXT NOT NULL,
	status TEXT NOT NULL,
	pid INTEGER,
	rows INTEGER NOT NULL,
	cols INTEGER NOT NULL,
	exit_code INTEGER,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL,
	closed_at datetime
);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_workspace_id ON terminal_sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_status ON terminal_sessions(status);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_updated_at ON terminal_sessions(updated_at);
