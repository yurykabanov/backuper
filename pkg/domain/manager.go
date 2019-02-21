package domain

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type backupService interface {
	StartBackup(context.Context, Rule) (Backup, error)
	FinishBackup(context.Context, Backup) (Backup, error)
	AbortBackup(context.Context, Backup) error
	DeleteBackup(context.Context, Backup) error
}

type BackupManager struct {
	logger logrus.FieldLogger

	rules  map[string]Rule
	active map[string]chan Backup

	service backupService
	repo    BackupRepository

	cron Cron
}

func NewBackupManager(
	logger logrus.FieldLogger,
	rr []Rule,
	service backupService,
	repo BackupRepository,
	cron Cron,
) *BackupManager {
	active := make(map[string]chan Backup, len(rr))
	rules := make(map[string]Rule)

	for _, rule := range rr {
		rules[rule.Name] = rule
		active[rule.Name] = make(chan Backup, 1)
	}

	return &BackupManager{
		logger: logger,

		rules:  rules,
		active: active,

		service: service,
		repo:    repo,

		cron: cron,
	}
}

type Cron interface {
	AddFunc(spec string, cmd func()) error
	Start()
}

func (m *BackupManager) Run() {
	// find all unfinished backups
	backups, err := m.repo.FindAllUnfinished(context.Background())
	if err != nil {
		m.logger.Fatal(err)
	}

	if len(backups) > 0 {
		m.logger.WithField("total", len(backups)).Info("Trying to continue managing unfinished backups")
	}

	// throw them into `m.active`
	for _, b := range backups {
		go func(b Backup) {
			if ch, ok := m.active[b.Rule]; ok {
				m.logger.WithFields(logrus.Fields{"id": b.Id, "rule": b.Rule}).Debug("Resuming backup", "id")
				ch <- b
				return
			}

			m.logger.Warn("Aborting backup due to rule became unavailable", "id", b.Id, "rule", b.Rule)

			err = m.service.AbortBackup(context.TODO(), b)
			if err != nil {
				m.logger.Error(err)
			}
		}(b)
	}

	// register handlers in go cron for every rule
	for rule, ch := range m.active {
		err = m.cron.AddFunc(m.rules[rule].CronSpec, func() {
			t := time.Now()

			fields := logrus.Fields{"rule": rule, "time": t}

			select {
			case ch <- Backup{Rule: rule, CreatedAt: t}:
				m.logger.WithFields(fields).Info("Dispatched new backup")
			default:
				m.logger.WithFields(fields).Warn("Unable to dispatch new backup")
			}
		})
		if err != nil {
			m.logger.WithField("spec", m.rules[rule].CronSpec).Fatalf("Invalid cron spec: '%s'", m.rules[rule].CronSpec)
		}
	}

	m.logger.Debug("Starting cron")
	m.cron.Start()

	wg := &sync.WaitGroup{}
	wg.Add(len(m.active))

	// start goroutines for each rule & chan from `m.active`
	for rule, ch := range m.active {
		go m.handleRuleBackups(wg, m.rules[rule], ch)
	}

	wg.Wait()
}

func (m *BackupManager) handleRuleBackups(wg *sync.WaitGroup, rule Rule, ch <-chan Backup) {
	m.logger.WithFields(logrus.Fields{"rule": rule.Name, "spec": rule.CronSpec}).Debug("Starting handler")

	for backup := range ch {
		var err error

		m.logger.WithField("rule", rule.Name).Info("Handling new backup task")

		// for new backups: perform `service.StartBackup`
		if backup.ExecStatus == ExecStatusNew {
			ctx, cancel := context.WithTimeout(context.Background(), rule.Timeout)

			backup, err = m.service.StartBackup(ctx, rule)
			if err != nil {
				m.logger.WithError(err).Error("Unable to start backup")
				continue
			}

			cancel()
		}

		m.logger.WithFields(logrus.Fields{"rule": rule.Name, "container_id": backup.ContainerId}).Info("Awaiting backup to finish")

		// for both new and previously unfinished backups: perform `service.FinishBackup`
		ctx, cancel := context.WithDeadline(context.Background(), backup.CreatedAt.Add(rule.Timeout))

		backup, err = m.service.FinishBackup(ctx, backup)
		if err != nil {
			m.logger.WithError(err).Error("Unable to finish backup")
		}

		cancel()

		m.logger.WithFields(logrus.Fields{"rule": rule.Name, "container_id": backup.ContainerId, "status_code": backup.StatusCode}).Info("Backup finished")

		m.logger.WithFields(logrus.Fields{"rule": rule.Name}).Info("Sweeping old backups")
		recentSuccessfulBackups, err := m.repo.FindAllSuccessfulNotDeleted(context.Background(), rule)
		if err != nil {
			m.logger.WithError(err).Error("Unable to query old backups")
		}

		// if rule limit max amount of backups: perform `service.DeleteBackup` for old ones
		if rule.PreserveAtMost <= 0 || len(recentSuccessfulBackups) <= rule.PreserveAtMost {
			continue
		}

		for _, b := range recentSuccessfulBackups[rule.PreserveAtMost:] {
			err = m.service.DeleteBackup(context.Background(), b)
			if err != nil {
				m.logger.WithError(err).Error("Unable to delete backup")
			}
		}
	}

	wg.Done()
}
