package domainfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(LoadRules),
	fx.Provide(NewCron),
	fx.Provide(MountManagerConfigProvider),
	fx.Provide(MountManager),
	fx.Provide(TransferManagerConfigProvider),
	fx.Provide(TransferManager),
	fx.Provide(BackupService),
	fx.Provide(BackupManager),
	fx.Invoke(RunBackupManager),
)
