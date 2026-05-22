CREATE TABLE conversations (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	title TEXT NOT NULL,
	last_message_at datetime NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NOT NULL
);
INSERT INTO conversations(id, workspace_id, title, last_message_at, created_at, updated_at)
SELECT id, workspace_id, title, updated_at, created_at, updated_at FROM sessions;
INSERT OR IGNORE INTO conversations(id, workspace_id, title, last_message_at, created_at, updated_at)
SELECT
	'conv_' || substr(id, 6),
	workspace_id,
	substr(prompt, 1, 80),
	created_at,
	created_at,
	updated_at
FROM agent_tasks
WHERE session_id IS NULL;
DROP TABLE sessions;
CREATE INDEX idx_conversations_workspace_id ON conversations(workspace_id);
CREATE INDEX idx_conversations_last_message_at ON conversations(last_message_at);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at);

CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	run_id TEXT,
	created_at datetime NOT NULL
);
INSERT INTO messages(id, workspace_id, conversation_id, role, content, run_id, created_at)
SELECT
	'msg_' || substr(id, 6),
	workspace_id,
	COALESCE(session_id, 'conv_' || substr(id, 6)),
	'user',
	prompt,
	'run_' || substr(id, 6),
	created_at
FROM agent_tasks;
CREATE INDEX idx_messages_workspace_id ON messages(workspace_id);
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_role ON messages(role);
CREATE INDEX idx_messages_run_id ON messages(run_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);

CREATE TABLE agent_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	trigger_message_id TEXT NOT NULL,
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
INSERT INTO agent_runs(
	id,
	workspace_id,
	conversation_id,
	trigger_message_id,
	model,
	reasoning_effort,
	status,
	summary,
	error,
	started_at,
	finished_at,
	created_at,
	updated_at
)
SELECT
	'run_' || substr(id, 6),
	workspace_id,
	COALESCE(session_id, 'conv_' || substr(id, 6)),
	'msg_' || substr(id, 6),
	model,
	reasoning_effort,
	status,
	summary,
	error,
	started_at,
	finished_at,
	created_at,
	updated_at
FROM agent_tasks;
DROP TABLE agent_tasks;
CREATE INDEX idx_agent_runs_workspace_id ON agent_runs(workspace_id);
CREATE INDEX idx_agent_runs_conversation_id ON agent_runs(conversation_id);
CREATE INDEX idx_agent_runs_trigger_message_id ON agent_runs(trigger_message_id);
CREATE INDEX idx_agent_runs_status ON agent_runs(status);
CREATE INDEX idx_agent_runs_created_at ON agent_runs(created_at);
CREATE INDEX idx_agent_runs_updated_at ON agent_runs(updated_at);

CREATE TABLE agent_run_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	type TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at datetime NOT NULL
);
INSERT INTO agent_run_events(id, workspace_id, run_id, type, payload_json, created_at)
SELECT id, workspace_id, 'run_' || substr(task_id, 6), type, payload_json, created_at
FROM agent_task_events;
DROP TABLE agent_task_events;
CREATE INDEX idx_agent_run_events_workspace_id ON agent_run_events(workspace_id);
CREATE INDEX idx_agent_run_events_run_id ON agent_run_events(run_id);
CREATE INDEX idx_agent_run_events_type ON agent_run_events(type);
CREATE INDEX idx_agent_run_events_created_at ON agent_run_events(created_at);

CREATE TABLE agent_tool_calls_new (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
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
INSERT INTO agent_tool_calls_new(
	id,
	workspace_id,
	run_id,
	batch_id,
	sequence,
	provider_call_id,
	name,
	input_json,
	output_json,
	status,
	requires_approval,
	decision,
	started_at,
	finished_at,
	created_at
)
SELECT
	id,
	workspace_id,
	'run_' || substr(task_id, 6),
	batch_id,
	sequence,
	provider_call_id,
	name,
	input_json,
	output_json,
	status,
	requires_approval,
	decision,
	started_at,
	finished_at,
	created_at
FROM agent_tool_calls;
DROP TABLE agent_tool_calls;
ALTER TABLE agent_tool_calls_new RENAME TO agent_tool_calls;
CREATE INDEX idx_agent_tool_calls_workspace_id ON agent_tool_calls(workspace_id);
CREATE INDEX idx_agent_tool_calls_run_id ON agent_tool_calls(run_id);
CREATE INDEX idx_agent_tool_calls_batch_id ON agent_tool_calls(batch_id);
CREATE INDEX idx_agent_tool_calls_status ON agent_tool_calls(status);
CREATE INDEX idx_agent_tool_calls_created_at ON agent_tool_calls(created_at);

CREATE TABLE commands_new (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	run_id TEXT,
	command TEXT NOT NULL,
	cwd TEXT NOT NULL,
	status TEXT NOT NULL,
	exit_code INTEGER,
	started_at datetime,
	finished_at datetime,
	created_at datetime NOT NULL
);
INSERT INTO commands_new(id, workspace_id, run_id, command, cwd, status, exit_code, started_at, finished_at, created_at)
SELECT
	id,
	workspace_id,
	CASE WHEN task_id IS NULL THEN NULL ELSE 'run_' || substr(task_id, 6) END,
	command,
	cwd,
	status,
	exit_code,
	started_at,
	finished_at,
	created_at
FROM commands;
DROP TABLE commands;
ALTER TABLE commands_new RENAME TO commands;
CREATE INDEX idx_commands_workspace_id ON commands(workspace_id);
CREATE INDEX idx_commands_status ON commands(status);
CREATE INDEX idx_commands_created_at ON commands(created_at);
