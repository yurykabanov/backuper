ALTER TABLE backups ADD COLUMN backup_size INT NOT NULL DEFAULT 0;
ALTER TABLE backups ADD COLUMN finished_at TIMESTAMP;
