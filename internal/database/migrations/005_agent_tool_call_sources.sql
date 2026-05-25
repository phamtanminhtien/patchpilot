ALTER TABLE agent_tool_calls ADD COLUMN source TEXT NOT NULL DEFAULT 'builtin';
ALTER TABLE agent_tool_calls ADD COLUMN source_ref TEXT;
ALTER TABLE agent_tool_calls ADD COLUMN policy_reason TEXT NOT NULL DEFAULT '';
