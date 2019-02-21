package domain

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
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
}

type MountManager interface {
	Allocate() (string, error)
	Deallocate(string) error
}

type DockerClient interface {
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
	repo            BackupRepository
	docker          DockerClient
	mountManager    MountManager
	transferManager TransferManager
}

func NewBackupService(
	repo BackupRepository,
	docker DockerClient,
	mountManager MountManager,
	transferManager TransferManager,
) *BackupService {
	return &BackupService{
		repo:            repo,
		docker:          docker,
		mountManager:    mountManager,
		transferManager: transferManager,
	}
}

func (s *BackupService) StartBackup(ctx context.Context, rule Rule) (Backup, error) {
	var backup Backup
	var err error

	defer func() {
		if err != nil {
			backup.ExecStatus = ExecStatusFailure

			_ = s.repo.Update(context.Background(), backup)
		}
	}()

	backup = Backup{
		Rule:            rule.Name,
		TargetDirectory: rule.TargetDirectory,
		ExecStatus:      ExecStatusCreated,
		CreatedAt:       time.Now(),
	}

	ref, err := reference.ParseNormalizedNamed(rule.Image)
	if err != nil {
		return backup, err
	}

	err = s.pullImage(ctx, ref)
	if err != nil {
		return backup, err
	}

	dir, err := s.mountManager.Allocate()
	if err != nil {
		return backup, err
	}

	backup.TempDirectory = dir

	backup, err = s.repo.Create(context.Background(), backup)
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
		},                           // container config
		&container.HostConfig{
			NetworkMode: "host",
			Mounts: []mount.Mount{
				{Type: mount.TypeBind, Source: dir, Target: "/__backup__"},
			},
		},                           // host config
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
	var status int64
	var err error

	defer func() {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

		_ = s.docker.ContainerRemove(ctx, backup.ContainerId, types.ContainerRemoveOptions{
			Force: true,
		})
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

	dir, err := s.transferManager.Transfer(backup)
	if err != nil {
		_, _ = s.markWithStatusAndDeallocate(backup, ExecStatusFailure)

		return backup, err
	}
	backup.BackupDirectory = dir

	return s.markWithStatusAndDeallocate(backup, ExecStatusSuccess)
}

func (s *BackupService) AbortBackup(ctx context.Context, backup Backup) error {
	_ = s.docker.ContainerRemove(ctx, backup.ContainerId, types.ContainerRemoveOptions{
		Force: true,
	})

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
	backup.ExecStatus = execStatus

	err := s.repo.Update(context.Background(), backup)
	if err != nil {
		return backup, err
	}
	err = s.mountManager.Deallocate(backup.TempDirectory)
	if err != nil {
		return backup, err
	}

	return backup, nil
}

func (s *BackupService) DeleteBackup(ctx context.Context, backup Backup) error {
	err := s.mountManager.Deallocate(backup.BackupDirectory)
	if err != nil {
		return err
	}

	now := time.Now()
	backup.DeletedAt = &now

	err = s.repo.Update(ctx, backup)

	return err
}
