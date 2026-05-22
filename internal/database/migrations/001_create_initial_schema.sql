CREATE TABLE IF NOT EXISTS app_metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	root_path TEXT NOT NULL,
	git_remote TEXT,
	default_branch TEXT,
	status TEXT NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_root_path ON workspaces(root_path);
CREATE INDEX IF NOT EXISTS idx_workspaces_updated_at ON workspaces(updated_at);

CREATE TABLE IF NOT EXISTS file_index (
	workspace_id TEXT NOT NULL,
	path TEXT NOT NULL,
	size INTEGER NOT NULL,
	modified_at datetime NOT NULL,
	indexed_at datetime NOT NULL,
	PRIMARY KEY (workspace_id, path)
);
CREATE INDEX IF NOT EXISTS idx_file_index_indexed_at ON file_index(indexed_at);

CREATE TABLE IF NOT EXISTS auth_sessions (
	id TEXT PRIMARY KEY,
	session_hash TEXT NOT NULL,
	created_at datetime NOT NULL,
	last_seen_at datetime NOT NULL,
	expires_at datetime NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_session_hash ON auth_sessions(session_hash);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_last_seen_at ON auth_sessions(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	title TEXT NOT NULL,
	mode TEXT NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_workspace_id ON sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at);

CREATE TABLE IF NOT EXISTS agent_tasks (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	session_id TEXT,
	prompt TEXT NOT NULL,
	model TEXT NOT NULL,
	reasoning_effort TEXT NOT NULL,
	status TEXT NOT NULL,
	summary TEXT NOT NULL,
	error TEXT,
	started_at datetime,
	finished_at datetime,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_workspace_id ON agent_tasks(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_status ON agent_tasks(status);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_created_at ON agent_tasks(created_at);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_updated_at ON agent_tasks(updated_at);

CREATE TABLE IF NOT EXISTS agent_task_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	type TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_workspace_id ON agent_task_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_task_id ON agent_task_events(task_id);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_type ON agent_task_events(type);
CREATE INDEX IF NOT EXISTS idx_agent_task_events_created_at ON agent_task_events(created_at);

CREATE TABLE IF NOT EXISTS agent_tool_calls (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	batch_id TEXT NOT NULL,
	sequence INTEGER NOT NULL,
	provider_call_id TEXT NOT NULL,
	name TEXT NOT NULL,
	input_json TEXT NOT NULL,
	output_json TEXT NOT NULL,
	status TEXT NOT NULL,
	requires_approval numeric NOT NULL,
	decision TEXT,
	started_at datetime,
	finished_at datetime,
	created_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_workspace_id ON agent_tool_calls(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_task_id ON agent_tool_calls(task_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_batch_id ON agent_tool_calls(batch_id);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_status ON agent_tool_calls(status);
CREATE INDEX IF NOT EXISTS idx_agent_tool_calls_created_at ON agent_tool_calls(created_at);

CREATE TABLE IF NOT EXISTS commands (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	task_id TEXT,
	command TEXT NOT NULL,
	cwd TEXT NOT NULL,
	status TEXT NOT NULL,
	exit_code INTEGER,
	started_at datetime,
	finished_at datetime,
	created_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_commands_workspace_id ON commands(workspace_id);
CREATE INDEX IF NOT EXISTS idx_commands_status ON commands(status);
CREATE INDEX IF NOT EXISTS idx_commands_created_at ON commands(created_at);

CREATE TABLE IF NOT EXISTS command_output (
	id TEXT PRIMARY KEY,
	command_id TEXT NOT NULL,
	stream TEXT NOT NULL,
	chunk TEXT NOT NULL,
	created_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_command_output_command_id ON command_output(command_id);
CREATE INDEX IF NOT EXISTS idx_command_output_created_at ON command_output(created_at);

CREATE TABLE IF NOT EXISTS ports (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	process_id TEXT,
	port INTEGER NOT NULL,
	status TEXT NOT NULL,
	exposed_path TEXT,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL,
	closed_at datetime
);
CREATE INDEX IF NOT EXISTS idx_ports_workspace_id ON ports(workspace_id);
CREATE INDEX IF NOT EXISTS idx_ports_process_id ON ports(process_id);
CREATE INDEX IF NOT EXISTS idx_ports_port ON ports(port);
CREATE INDEX IF NOT EXISTS idx_ports_status ON ports(status);
CREATE INDEX IF NOT EXISTS idx_ports_updated_at ON ports(updated_at);

CREATE TABLE IF NOT EXISTS git_snapshots (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	commit_sha TEXT,
	status_json TEXT NOT NULL,
	created_at datetime NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_git_snapshots_workspace_id ON git_snapshots(workspace_id);
CREATE INDEX IF NOT EXISTS idx_git_snapshots_commit_sha ON git_snapshots(commit_sha);
CREATE INDEX IF NOT EXISTS idx_git_snapshots_created_at ON git_snapshots(created_at);
