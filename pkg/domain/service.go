package domain

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/yurykabanov/backuper/pkg/appcontext"
	"github.com/yurykabanov/backuper/pkg/util"
)

const maxErrorsWhileFinishing = 100

type BackupRepository interface {
	Create(context.Context, Backup) (Backup, error)
	Update(context.Context, Backup) error
	FindAllUnfinished(context.Context) ([]Backup, error)
	FindAllSuccessfulNotDeleted(context.Context, Rule) ([]Backup, error)
}

type TransferManager interface {
	Transfer(Backup) (string, error)
	Remove(Backup) error
}

type MountManager interface {
	AllocateTemp() (string, error)
	DeallocateTemp(string) error
}

type dockerClient interface {
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		containerName string,
	) (container.ContainerCreateCreatedBody, error)

	ContainerStart(
		ctx context.Context,
		containerID string,
		options types.ContainerStartOptions,
	) error

	ContainerWait(
		ctx context.Context,
		containerID string,
	) (int64, error)

	ContainerRemove(
		ctx context.Context,
		containerID string,
		options types.ContainerRemoveOptions,
	) error

	ImagePull(
		ctx context.Context,
		ref string,
		options types.ImagePullOptions,
	) (io.ReadCloser, error)
}

type BackupService struct {
	logger logrus.FieldLogger

	repo            BackupRepository
	docker          dockerClient
	mountManager    MountManager
	transferManager TransferManager
}

func NewBackupService(
	logger logrus.FieldLogger,
	repo BackupRepository,
	docker dockerClient,
	mountManager MountManager,
	transferManager TransferManager,
) *BackupService {
	return &BackupService{
		logger:          logger,
		repo:            repo,
		docker:          docker,
		mountManager:    mountManager,
		transferManager: transferManager,
	}
}

func (s *BackupService) StartBackup(ctx context.Context, rule Rule) (Backup, error) {
	logger := appcontext.LoggerFromContext(s.logger, ctx)

	var backup Backup
	var err error

	defer func() {
		if err != nil {
			logger.Debug("BackupService::StartBackup finished with error, marking backup failed")

			backup.ExecStatus = ExecStatusFailure

			if err := s.repo.Update(context.Background(), backup); err != nil {
				logger.WithError(err).Error("BackupService::StartBackup is unable to mark backup failed")
			}
		}
	}()

	backup = Backup{
		Rule:        rule.Name,
		ExecStatus:  ExecStatusCreated,
		CreatedAt:   time.Now(),
		StorageName: rule.StorageName,
	}

	ref, err := reference.ParseNormalizedNamed(rule.Image)
	if err != nil {
		return backup, err
	}

	err = s.pullImage(ctx, ref)
	if err != nil {
		return backup, err
	}

	dir, err := s.mountManager.AllocateTemp()
	if err != nil {
		return backup, err
	}

	backup.TempDirectory = dir

	backup, err = s.repo.Create(ctx, backup)
	if err != nil {
		return backup, err
	}

	c, err := s.docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: ref.String(),
			Cmd:   rule.Command,
			Env: []string{
				"BACKUP_TARGET_DIR=/__backup__",
			},
		}, // container config
		&container.HostConfig{
			NetworkMode: "host",
			Mounts: []mount.Mount{
				{Type: mount.TypeBind, Source: dir, Target: "/__backup__"},
			},
		}, // host config
		&network.NetworkingConfig{}, // networking config
		s.containerName(backup),
	)
	if err != nil {
		return backup, err
	}

	err = s.docker.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	if err != nil {
		return backup, err
	}

	backup.ExecStatus = ExecStatusStarted
	backup.ContainerId = c.ID

	err = s.repo.Update(context.Background(), backup)
	if err != nil {
		return backup, err
	}

	return backup, nil
}

func (s *BackupService) FinishBackup(ctx context.Context, backup Backup) (Backup, error) {
	logger := appcontext.LoggerFromContext(s.logger, ctx)

	var status int64
	var err error

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

		if err := s.docker.ContainerRemove(ctx, backup.ContainerId, types.ContainerRemoveOptions{Force: true}); err != nil {
			logger.WithError(err).Error("BackupService::FinishBackup is unable to remove container")
		}

		cancel()
	}()

	errCounter := 0
	for {
		status, err = s.docker.ContainerWait(ctx, backup.ContainerId)
		if err == nil {
			break
		}

		if err == context.DeadlineExceeded {
			_, _ = s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

			return backup, err
		}

		errCounter++
		if errCounter > maxErrorsWhileFinishing {
			break
		}
	}

	backup.StatusCode = status

	if status != 0 {
		_, _ = s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

		return backup, errors.New("status code is not zero")
	}

	tempBackupFile := path.Join(backup.TempDirectory, "__backup__.zip")
	err = util.ZipDirectory(tempBackupFile, backup.TempDirectory)
	if err != nil {
		_, _ = s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

		return backup, errors.New("unable to zip temp data")
	}
	backup.TempBackupFile = tempBackupFile

	if tmpFileStat, err := os.Stat(tempBackupFile); err == nil {
		backup.BackupSize = tmpFileStat.Size()
	} else {
		logger.WithError(err).Warn("Unable to calculate backup size in spite of it has finished successfully")
	}

	storageBackupFile, err := s.transferManager.Transfer(backup)
	if err != nil {
		_, _ = s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

		return backup, err
	}
	backup.BackupFile = storageBackupFile

	return s.markWithStatusAndDeallocate(backup, ExecStatusSuccess)
}

func (s *BackupService) AbortBackup(ctx context.Context, backup Backup) error {
	logger := appcontext.LoggerFromContext(s.logger, ctx)

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

		if err := s.docker.ContainerRemove(ctx, backup.ContainerId, types.ContainerRemoveOptions{Force: true}); err != nil {
			logger.WithError(err).Error("BackupService::FinishBackup is unable to remove container")
		}

		cancel()
	}()

	_, err := s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

	return err
}

func (s *BackupService) pullImage(ctx context.Context, ref reference.Named) error {
	img, err := s.docker.ImagePull(
		ctx,
		ref.String(),
		types.ImagePullOptions{},
	)
	if err != nil {
		return err
	}

	_, err = io.Copy(ioutil.Discard, img)
	if err != nil {
		return err
	}

	return nil
}

func (s *BackupService) containerName(backup Backup) string {
	return fmt.Sprintf("backup-%s-%d", backup.Rule, backup.Id)
}

func (s *BackupService) markWithStatusAndDeallocate(backup Backup, execStatus execStatus) (Backup, error) {
	now := time.Now()

	backup.ExecStatus = execStatus
	backup.FinishedAt = &now

	err := s.repo.Update(context.Background(), backup)
	if err != nil {
		return backup, err
	}
	err = s.mountManager.DeallocateTemp(backup.TempDirectory)
	if err != nil {
		return backup, err
	}

	return backup, nil
}

func (s *BackupService) DeleteBackup(ctx context.Context, backup Backup) error {
	err := s.transferManager.Remove(backup)
	if err != nil {
		return err
	}

	now := time.Now()
	backup.DeletedAt = &now

	err = s.repo.Update(ctx, backup)

	return err
}
