package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/yurykabanov/backuper/pkg/appcontext"
	"github.com/yurykabanov/backuper/pkg/domain"
)

type BackupRepository interface {
	FindLastSuccessful(context.Context) ([]domain.Backup, error)
}

type BackupMetricHandler struct {
	logger logrus.FieldLogger
	rules  []domain.Rule
	repo   BackupRepository
}

func NewBackupMetricHandler(logger logrus.FieldLogger, rules []domain.Rule, repo BackupRepository) *BackupMetricHandler {
	return &BackupMetricHandler{
		logger: logger,
		rules:  rules,
		repo:   repo,
	}
}

type backupMetricResponse struct {
	RuleName               string    `json:"rule_name"`
	BackupSize             int64     `json:"backup_size"`
	LastSuccessfulAt       time.Time `json:"last_successful_at"`
	LastCompletionNanoTime int64     `json:"last_completion_nano_time"`
}

func (h *BackupMetricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	logger := appcontext.LoggerFromContext(h.logger, ctx)

	bb, err := h.repo.FindLastSuccessful(ctx)
	if err != nil {
		logger.WithError(err).Error("Unable to query last successful backups")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var result []backupMetricResponse

	for _, b := range bb {
		result = append(result, backupMetricResponse{
			RuleName:               b.Rule,
			LastSuccessfulAt:       b.CreatedAt,
			LastCompletionNanoTime: b.FinishedAt.Sub(b.CreatedAt).Nanoseconds(),
			BackupSize:             b.BackupSize,
		})
	}

	enc := json.NewEncoder(w)
	err = enc.Encode(result)
	if err != nil {
		logger.WithError(err).Error("Unable to encode response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}
