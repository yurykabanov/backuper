package domainfx

import (
	"context"

	docker "github.com/docker/docker/client"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/yurykabanov/go-yandex-disk"
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

type TransferManagerConfig struct {
	NamedEntries map[string]TransferManagerConfigEntry
}

type TransferManagerConfigEntry struct {
	Type string
	Root string
	Opts map[string]interface{}
}

func TransferManagerConfigProvider(v *viper.Viper) (*TransferManagerConfig, error) {
	var config map[string]TransferManagerConfigEntry

	err := v.UnmarshalKey("transfer", &config)
	if err != nil {
		return nil, err
	}

	return &TransferManagerConfig{NamedEntries: config}, nil
}

func TransferManager(config *TransferManagerConfig) domain.TransferManager {
	var mounts = make(map[string]domain.TransferManager)

	for k, v := range config.NamedEntries {
		var m domain.TransferManager
		switch v.Type {
		case "local":
			m = transfer.NewLocalMount(v.Root)
		case "yadisk":
			m = transfer.NewYaDiskMount(yadisk.NewFromAccessToken(v.Opts["access_token"].(string)), v.Root)
		default:
			continue
		}
		mounts[k] = m
	}

	return transfer.NewManager(mounts)
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
