CREATE TABLE IF NOT EXISTS agent_sessions (
  id VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  title VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_messages (
  id VARCHAR(64) PRIMARY KEY,
  session_id VARCHAR(64) NOT NULL,
  role VARCHAR(32) NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_agent_messages_session_created (session_id, created_at)
);

CREATE TABLE IF NOT EXISTS agent_tool_calls (
  call_id VARCHAR(64) PRIMARY KEY,
  session_id VARCHAR(64) NOT NULL,
  message_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(128) NOT NULL,
  tool_name VARCHAR(128) NOT NULL,
  input_json JSON NOT NULL,
  sanitized_input_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  duration_ms BIGINT NOT NULL,
  error_message TEXT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_agent_tool_calls_created (created_at),
  INDEX idx_agent_tool_calls_tool (tool_name)
);

CREATE TABLE IF NOT EXISTS policy_reject_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  session_id VARCHAR(64) NOT NULL,
  tool_name VARCHAR(128) NOT NULL,
  reason VARCHAR(255) NOT NULL,
  input_json JSON NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_policy_reject_logs_created (created_at),
  INDEX idx_policy_reject_logs_tool (tool_name)
);

CREATE TABLE IF NOT EXISTS documents (
  id VARCHAR(64) PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  source_type VARCHAR(64) NOT NULL,
  source_path VARCHAR(512) NOT NULL,
  status VARCHAR(32) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS document_chunks (
  id VARCHAR(64) PRIMARY KEY,
  document_id VARCHAR(64) NOT NULL,
  chunk_index INT NOT NULL,
  content TEXT NOT NULL,
  token_count INT NOT NULL,
  qdrant_point_id VARCHAR(128) NOT NULL,
  metadata JSON NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_document_chunks_document (document_id),
  FULLTEXT INDEX ft_document_chunks_content (content)
);

CREATE TABLE IF NOT EXISTS async_tasks (
  task_id VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  tool_name VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL,
  input_json JSON NOT NULL,
  result_json JSON NULL,
  error_message TEXT NULL,
  retry_count INT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  finished_at DATETIME NULL,
  INDEX idx_async_tasks_status (status),
  INDEX idx_async_tasks_user (user_id)
);
