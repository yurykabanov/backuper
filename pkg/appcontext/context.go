package appcontext

import (
	"context"

	"github.com/sirupsen/logrus"
)

type contextId int

const (
	ruleNameKeyId contextId = iota
	containerIdKeyId
	backupIdKeyId
	requestIdKeyId
)

func WithRequestId(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, requestIdKeyId, requestId)
}

func WithBackupId(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, backupIdKeyId, id)
}

func WithRuleName(ctx context.Context, rule string) context.Context {
	return context.WithValue(ctx, ruleNameKeyId, rule)
}

func WithContainerId(ctx context.Context, containerId string) context.Context {
	return context.WithValue(ctx, containerIdKeyId, containerId)
}

func LoggerFromContext(logger logrus.FieldLogger, ctx context.Context) logrus.FieldLogger {
	if ctx == nil {
		return logger
	}

	result := logger

	if ctxRuleName, ok := ctx.Value(ruleNameKeyId).(string); ok {
		result = result.WithField("rule", ctxRuleName)
	}

	if ctxContainerId, ok := ctx.Value(containerIdKeyId).(string); ok && ctxContainerId != "" {
		result = result.WithField("container_id", ctxContainerId)
	}

	if ctxBackupId, ok := ctx.Value(backupIdKeyId).(int64); ok && ctxBackupId != 0 {
		result = result.WithField("backup_id", ctxBackupId)
	}

	if ctxRequestId, ok := ctx.Value(requestIdKeyId).(string); ok && ctxRequestId != "" {
		result = result.WithField("request_id", ctxRequestId)
	}

	return result
}
