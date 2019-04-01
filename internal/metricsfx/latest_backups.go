package metricsfx

import (
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/yurykabanov/backuper/pkg/domain"
	"github.com/yurykabanov/backuper/pkg/http/handler"
)

func LatestBackupMetricHandler(
	logger *logrus.Logger,
	rules []domain.Rule,
	repository handler.BackupRepository,
) *handler.BackupMetricHandler {
	return handler.NewBackupMetricHandler(logger, rules, repository)
}

func RegisterLatestBackupMetricHandler(router *mux.Router, h *handler.BackupMetricHandler) {
	router.Handle("/metrics/backups", h)
}
