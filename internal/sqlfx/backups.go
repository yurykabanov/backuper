package sqlfx

import (
	"github.com/jmoiron/sqlx"

	"github.com/yurykabanov/backuper/pkg/domain"
	"github.com/yurykabanov/backuper/pkg/http/handler"
	"github.com/yurykabanov/backuper/pkg/storage"
)

func BackupsRepository(db *sqlx.DB) (
	*storage.BackupRepository,
	domain.BackupRepository,
	handler.BackupRepository,
) {
	repo := storage.NewBackupRepository(db)

	return repo, repo, repo
}
