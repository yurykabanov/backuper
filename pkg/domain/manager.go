package domain

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/yurykabanov/backuper/pkg/appcontext"
)

// Backup manager is the core of the backuper. It manages backup rules,
// run them using provided schedule, resumes unfinished backups after
// restart etc.
type BackupManager struct {
	logger logrus.FieldLogger

	rules  map[string]Rule
	active map[string]chan Backup

	service backupService
	repo    BackupRepository

	cron cron
}

func NewBackupManager(
	logger logrus.FieldLogger,
	rules []Rule,
	service backupService,
	repo BackupRepository,
	cron cron,
) *BackupManager {
	active := make(map[string]chan Backup, len(rules))
	rulesMap := make(map[string]Rule)

	for _, rule := range rules {
		rulesMap[rule.Name] = rule
		active[rule.Name] = make(chan Backup, 1)
	}

	return &BackupManager{
		logger: logger,

		rules:  rulesMap,
		active: active,

		service: service,
		repo:    repo,

		cron: cron,
	}
}

type backupService interface {
	StartBackup(context.Context, Rule) (Backup, error)
	FinishBackup(context.Context, Backup) (Backup, error)
	AbortBackup(context.Context, Backup) error
	DeleteBackup(context.Context, Backup) error
}

type cron interface {
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
		m.logger.WithField("total_unfinished_backups", len(backups)).Info("Trying to continue managing unfinished backups")
	}

	// enqueue or abort unfinished backups
	for _, backup := range backups {
		go m.enqueueOrAbort(context.Background(), backup)
	}

	// register handlers in go cron for every rule
	for rule, ch := range m.active {
		err = m.registerRule(rule, ch)
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

func (m *BackupManager) enqueueOrAbort(ctx context.Context, backup Backup) {
	ctx = appcontext.WithContainerId(appcontext.WithBackupId(appcontext.WithRuleName(ctx, backup.Rule), backup.Id), backup.ContainerId)

	logger := appcontext.LoggerFromContext(m.logger, ctx)

	if ch, ok := m.active[backup.Rule]; ok {
		logger.Debug("Resuming backup")
		ch <- backup
		return
	}

	logger.Warn("Aborting backup due to rule became unavailable")

	err := m.service.AbortBackup(ctx, backup)
	if err != nil {
		logger.WithError(err).Error("Unable to abort backup")
	}
}

func (m *BackupManager) handleRuleBackups(wg *sync.WaitGroup, rule Rule, ch <-chan Backup) {
	baseCtx := appcontext.WithRuleName(context.Background(), rule.Name)
	logger := appcontext.LoggerFromContext(m.logger, baseCtx)

	logger.WithFields(logrus.Fields{"spec": rule.CronSpec}).Debug("Starting rule handler")

	for backup := range ch {
		m.handleRuleBackup(baseCtx, rule, backup)
	}

	wg.Done()
}

func (m *BackupManager) handleRuleBackup(ctx context.Context, rule Rule, backup Backup) {
	logger := appcontext.LoggerFromContext(m.logger, ctx)

	logger.Info("Handling new backup task")

	// for new backups: perform `service.StartBackup`
	if backup.ExecStatus == ExecStatusNew {
		backup = m.startBackup(ctx, rule, backup)
	}

	// for both new and previously unfinished backups: perform `service.FinishBackup`
	m.awaitBackupFinish(appcontext.WithBackupId(ctx, backup.Id), rule, backup)

	// sweep old backups if any
	m.sweepOldBackups(ctx, rule)
}

func (m *BackupManager) startBackup(ctx context.Context, rule Rule, backup Backup) Backup {
	ctx, cancel := context.WithTimeout(ctx, rule.Timeout)
	defer cancel()

	logger := appcontext.LoggerFromContext(m.logger, ctx)

	logger.Info("Starting new backup")
	backup, err := m.service.StartBackup(ctx, rule)
	if err != nil {
		logger.WithError(err).Error("Unable to start backup")
	}

	return backup
}

func (m *BackupManager) awaitBackupFinish(ctx context.Context, rule Rule, backup Backup) {
	ctx = appcontext.WithContainerId(ctx, backup.ContainerId)
	ctx, cancel := context.WithDeadline(ctx, backup.CreatedAt.Add(rule.Timeout))
	defer cancel()

	logger := appcontext.LoggerFromContext(m.logger, ctx)

	logger.Info("Awaiting backup to finish")
	backup, err := m.service.FinishBackup(ctx, backup)
	if err != nil {
		logger.WithError(err).Error("Unable to finish backup")
	}

	logger.WithField("status_code", backup.StatusCode).Info("Backup finished")
}

// Each generation is considered as following:
//
//             preserved
//             [--------]
//      -X--X--X--X--X--X----> t
//       [--]
//       old
//
// where 'X' is a backup made at some time moment
//
// Old items are candidates for pushing to the next generation
//
//     generation 0:                 --A1--[A2--A3--A4--A5]-->t
//                                     |
//     generation 1: --B1-----B2-------__
//
// On this figure one 'A' backup is such candidate.
//
// It could be either pushed or not depending on time diff between B2 and A1
//
//     generation 0:             A1--[A2--A3--A4--A5]-->t
//     generation 1: --B1-----B2-__                       <- A1 will be discarded, time_diff(A1, B2) is too small
//
//     generation 0:                 A1--[A2--A3--A4--A5]-->t
//     generation 1: --B1-----B2-----__                   <- A1 will not be discarded, time_diff(A1, B2) is enough
//
func (m *BackupManager) sweepOldBackups(ctx context.Context, rule Rule) {
	logger := appcontext.LoggerFromContext(m.logger, ctx)

	logger.Info("Sweeping old backups")
	recentSuccessfulBackups, err := m.repo.FindAllSuccessfulNotDeleted(ctx, rule)
	if err != nil {
		logger.WithError(err).Error("Unable to query old backups")
	}

	backups := m.groupByGeneration(recentSuccessfulBackups)
	maxGeneration := len(rule.RotationRules) - 1

	for generation := 0; generation <= maxGeneration; generation++ {
		// How many backups will are candidates for pushing to the next generation
		oldCount := len(backups[generation]) - rule.RotationRules[generation].PreserveAtMost

		// The `oldCount` value could be:
		// - negative: current generation is not full
		// - zero: generation is full, and there is no new backups
		// - positive: 1 or more backups should be pushed to next generation (more than one in case of resizing)
		if oldCount <= 0 {
			continue
		}

		oldBackups := backups[generation][:oldCount]
		backups[generation] = backups[generation][oldCount:]

		// Backups from last generation are discarded completely
		if generation >= maxGeneration {
			logger.Infof("Found %d backups with generation %d, discarding them completely", oldCount, generation)

			for _, backup := range oldBackups {
				err = m.service.DeleteBackup(appcontext.WithBackupId(ctx, backup.Id), backup)
				if err != nil {
					logger.WithError(err).Error("Unable to delete backup")
				}
			}

			continue
		}

		// Push all old backups from given generation to the next one
		for _, old := range oldBackups {
			isEmpty := len(backups[generation+1]) == 0
			if !isEmpty {
				diffToNewestFromNextGeneration := old.CreatedAt.Sub(backups[generation+1][len(backups[generation+1])-1].CreatedAt)

				// If item is not old enough (i.e. not enough time has passed to satisfy next generation's `Period` clause), then discard it
				if diffToNewestFromNextGeneration.Seconds() < rule.RotationRules[generation+1].Period.Seconds() {
					logger.Infof("Discarding backup id=%d (generation %d) due to time difference is not enough for pushing it to the next generation", old.Id, generation)

					err = m.service.DeleteBackup(appcontext.WithBackupId(ctx, old.Id), old)
					if err != nil {
						logger.WithError(err).Error("Unable to delete backup")
					}

					continue
				}
			}

			logrus.Infof("Pushing backup id=%d from generation %d to %d", old.Id, generation, generation+1)
			old.Generation += 1
			err = m.repo.Update(appcontext.WithBackupId(ctx, old.Id), old)
			if err != nil {
				logger.WithError(err).Error("Unable to update backup")
			}

			backups[generation+1] = append(backups[generation+1], old)
		}
	}
}

func (m *BackupManager) groupByGeneration(backups []Backup) map[int][]Backup {
	result := make(map[int][]Backup)

	for _, b := range backups {
		result[b.Generation] = append(result[b.Generation], b)
	}

	return result
}

func (m *BackupManager) registerRule(rule string, ch chan<- Backup) error {
	return m.cron.AddFunc(m.rules[rule].CronSpec, func() {
		t := time.Now()

		fields := logrus.Fields{"rule": rule, "created_at": t}

		select {
		case ch <- Backup{Rule: rule, CreatedAt: t}:
			m.logger.WithFields(fields).Info("Dispatched new backup")
		default:
			m.logger.WithFields(fields).Warn("Unable to dispatch new backup")
		}
	})
}
