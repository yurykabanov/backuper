package domainfx

import (
	"context"

	docker "github.com/docker/docker/client"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/yurykabanov/backuper/pkg/domain"
	"github.com/yurykabanov/backuper/pkg/mount"
	"github.com/yurykabanov/backuper/pkg/transfer"
)

const (
	ConfigMountTempDirectory = "mount.temp_directory"
)

func NewCron() *cron.Cron {
	return cron.New()
}

type MountManagerConfig struct {
	BaseDirectory string
}

func MountManagerConfigProvider(v *viper.Viper) *MountManagerConfig {
	return &MountManagerConfig{
		BaseDirectory: v.GetString(ConfigMountTempDirectory),
	}
}

func MountManager(config *MountManagerConfig) domain.MountManager {
	return mount.New(config.BaseDirectory)
}

func TransferManager() domain.TransferManager {
	return transfer.New()
}

func BackupService(
	logger *logrus.Logger,
	repository domain.BackupRepository,
	dockerClient *docker.Client,
	mountManager domain.MountManager,
	transferManager domain.TransferManager,
) *domain.BackupService {
	return domain.NewBackupService(logger, repository, dockerClient, mountManager, transferManager)
}

func BackupManager(
	logger *logrus.Logger,
	rules []domain.Rule,
	service *domain.BackupService,
	repository domain.BackupRepository,
	cron *cron.Cron,
) *domain.BackupManager {
	return domain.NewBackupManager(logger, rules, service, repository, cron)
}

func RunBackupManager(lc fx.Lifecycle, backupManager *domain.BackupManager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go backupManager.Run()
			return nil
		},
	})
}
