CREATE TABLE backups
(
  id               INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  rule             VARCHAR(255)    NOT NULL,
  container_id     VARCHAR(255)    NOT NULL,
  temp_directory   VARCHAR(255)    NOT NULL,
  target_directory VARCHAR(255)    NOT NULL,
  backup_directory VARCHAR(255)    NOT NULL,
  exec_status      INT             NOT NULL DEFAULT 0,
  status_code      INT             NOT NULL DEFAULT 0,
  created_at       TIMESTAMP       NOT NULL,
  deleted_at       TIMESTAMP       NULL
);

CREATE INDEX backups_exec_status_idx ON backups(exec_status);

CREATE INDEX backups_deleted_at_idx ON backups(deleted_at);
