package storage

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/yurykabanov/backuper/pkg/domain"
)

const (
	backupInsertQuery = `
		INSERT INTO backups (
			rule, container_id,
			temp_directory, target_directory, backup_directory,
			exec_status, status_code, created_at, deleted_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	backupUpdateQuery = `
		UPDATE backups SET
			rule = ?, container_id = ?,
			temp_directory = ?, target_directory = ?, backup_directory = ?,
			exec_status = ?, status_code = ?, created_at = ?, deleted_at = ?
		WHERE id = ?
	`

	backupSelectUnfinished = `
		SELECT 
			id,
			rule, container_id,
			temp_directory, target_directory, backup_directory,
			exec_status, status_code, created_at, deleted_at
		FROM backups
		WHERE exec_status IN (?)
	`

	backupSelectSuccessfulNotDeleted = `
		SELECT
			id,
			rule, container_id,
			temp_directory, target_directory, backup_directory,
			exec_status, status_code, created_at, deleted_at
		FROM backups
		WHERE rule = ? 
			AND exec_status = 4
			AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
)

type BackupRepository struct {
	db *sqlx.DB
}

func NewBackupRepository(db *sqlx.DB) *BackupRepository {
	return &BackupRepository{
		db: db,
	}
}

func (r *BackupRepository) Create(ctx context.Context, backup domain.Backup) (domain.Backup, error) {
	stmt, err := r.db.PrepareContext(ctx, backupInsertQuery)
	if err != nil {
		return backup, err
	}

	res, err := stmt.ExecContext(
		ctx,
		backup.Rule, backup.ContainerId,
		backup.TempDirectory, backup.TargetDirectory, backup.BackupDirectory,
		backup.ExecStatus, backup.StatusCode, backup.CreatedAt, backup.DeletedAt,
	)
	if err != nil {
		return backup, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return backup, err
	}

	backup.Id = id

	return backup, nil
}

func (r *BackupRepository) Update(ctx context.Context, backup domain.Backup) error {
	stmt, err := r.db.PrepareContext(ctx, backupUpdateQuery)
	if err != nil {
		return err
	}

	_, err = stmt.ExecContext(
		ctx,
		backup.Rule, backup.ContainerId,
		backup.TempDirectory, backup.TargetDirectory, backup.BackupDirectory,
		backup.ExecStatus, backup.StatusCode, backup.CreatedAt, backup.DeletedAt,
		backup.Id,
	)

	return err
}

func (r *BackupRepository) FindAllUnfinished(ctx context.Context) ([]domain.Backup, error) {
	query, args, err := sqlx.In(backupSelectUnfinished, domain.ExecStatusUnfinished)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	var backups []domain.Backup

	err = r.db.SelectContext(ctx, &backups, query, args...)
	if err != nil {
		return nil, err
	}

	return backups, nil
}

func (r *BackupRepository) FindAllSuccessfulNotDeleted(ctx context.Context, rule domain.Rule) ([]domain.Backup, error) {
	var backups []domain.Backup

	err := r.db.SelectContext(ctx, &backups, backupSelectSuccessfulNotDeleted, rule.Name)
	if err != nil {
		return nil, err
	}

	return backups, nil
}
