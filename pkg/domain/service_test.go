package domain

import (
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// region mapBackupRepository
type backupRepositoryMock struct {
	mock.Mock
}

func (m *backupRepositoryMock) Create(ctx context.Context, backup Backup) (Backup, error) {
	args := m.Called(ctx, backup)
	return args.Get(0).(Backup), args.Error(1)
}

func (m *backupRepositoryMock) Update(ctx context.Context, backup Backup) error {
	args := m.Called(ctx, backup)
	return args.Error(0)
}

func (m *backupRepositoryMock) FindAllUnfinished(ctx context.Context) ([]Backup, error) {
	args := m.Called(ctx)
	return args.Get(0).([]Backup), args.Error(1)
}

func (m *backupRepositoryMock) FindAllSuccessfulNotDeleted(ctx context.Context, rule Rule) ([]Backup, error) {
	args := m.Called(ctx, rule)
	return args.Get(0).([]Backup), args.Error(1)
}

// endregion

// region dockerClientMock
type dockerClientMock struct {
	mock.Mock
}

func (m *dockerClientMock) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	containerName string,
) (container.ContainerCreateCreatedBody, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, containerName)
	return args.Get(0).(container.ContainerCreateCreatedBody), args.Error(1)
}

func (m *dockerClientMock) ContainerStart(
	ctx context.Context,
	containerID string,
	options types.ContainerStartOptions,
) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *dockerClientMock) ContainerWait(
	ctx context.Context,
	containerID string,
) (int64, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *dockerClientMock) ContainerRemove(
	ctx context.Context,
	containerID string,
	options types.ContainerRemoveOptions,
) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *dockerClientMock) ImagePull(
	ctx context.Context,
	ref string,
	options types.ImagePullOptions,
) (io.ReadCloser, error) {
	args := m.Called(ctx, ref, options)

	if r := args.Get(0); r != nil {
		return r.(io.ReadCloser), args.Error(1)
	}

	return nil, args.Error(1)
}

// endregion

// region mountManagerMock
type mountManagerMock struct {
	mock.Mock
}

func (m *mountManagerMock) Allocate() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mountManagerMock) Deallocate(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

// endregion

// region transferManagerMock
type transferManagerMock struct {
	mock.Mock
}

func (m *transferManagerMock) Transfer(backup Backup) (string, error) {
	args := m.Called(backup)
	return args.String(0), args.Error(1)
}

// endregion

// region namedReference
type namedReference struct {
}

func (*namedReference) Name() string {
	return "name"
}

func (*namedReference) String() string {
	return "string"
}

// endregion

func discardLogger() *logrus.Logger {
	logger := logrus.New()
	logger.Out = ioutil.Discard

	return logger
}

// region Test: pullImage
func TestService_pullImage_Success(t *testing.T) {
	repo := &backupRepositoryMock{}

	dockerClient := &dockerClientMock{}

	dockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).
		Return(ioutil.NopCloser(strings.NewReader("some response")), nil)

	svc := NewBackupService(discardLogger(), repo, dockerClient, nil, nil)

	err := svc.pullImage(context.Background(), &namedReference{})

	assert.Nil(t, err)
}

func TestService_pullImage_Failure(t *testing.T) {
	repo := &backupRepositoryMock{}

	dockerClient := &dockerClientMock{}

	dockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).
		Return(io.ReadCloser(nil), context.DeadlineExceeded)

	svc := NewBackupService(discardLogger(), repo, dockerClient, nil, nil)

	err := svc.pullImage(context.Background(), &namedReference{})

	assert.Equal(t, context.DeadlineExceeded, err)
}

// endregion

// region Test: StartBackup
func TestService_StartBackup(t *testing.T) {
	repo := &backupRepositoryMock{}
	dockerClient := &dockerClientMock{}
	mountManager := &mountManagerMock{}
	transferManager := &transferManagerMock{}

	createdAt, _ := time.Parse(time.RFC3339, "2019-01-01T02:03:04Z")

	newBackup := Backup{
		Rule:            "some-rule",
		ExecStatus:      ExecStatusCreated,
		TempDirectory:   "/tmp/temp_dir/some_mount_directory",
		TargetDirectory: "/tmp/whatever/",
		CreatedAt:       createdAt,
	}

	rule := Rule{
		Name:            "some-rule",
		Image:           "whatever/image:1.2.3",
		TargetDirectory: "/tmp/whatever/",
		Command:         []string{"echo", "123", ">", "$BACKUP_TARGET_DIRECTORY/backup.dat"},
	}

	tempDirectory := "/tmp/temp_dir/some_mount_directory"
	containerId := "some-id"

	ctx := context.Background()

	dockerClient.On("ImagePull", ctx, mock.Anything, mock.Anything).
		Return(ioutil.NopCloser(strings.NewReader("some response")), nil)

	mountManager.On("Allocate").
		Return(tempDirectory, nil)

	repo.On("Create", ctx, mock.AnythingOfType("Backup")).Return(newBackup, nil)

	dockerClient.On("ContainerCreate", ctx,
		&container.Config{
			Image: "docker.io/whatever/image:1.2.3",
			Cmd:   rule.Command,
			Env: []string{
				"BACKUP_TARGET_DIR=/__backup__",
			},
		},
		&container.HostConfig{
			NetworkMode: "host",
			Mounts: []mount.Mount{
				{Type: mount.TypeBind, Source: tempDirectory, Target: "/__backup__"},
			},
		}, mock.Anything, mock.Anything,
	).Return(container.ContainerCreateCreatedBody{ID: containerId}, nil)

	dockerClient.On("ContainerStart", ctx, containerId, mock.Anything).
		Return(nil)

	backup := newBackup
	backup.ExecStatus = ExecStatusStarted
	backup.ContainerId = "some-id"

	repo.On("Update", ctx, backup).Return(nil)

	svc := NewBackupService(discardLogger(), repo, dockerClient, mountManager, transferManager)

	backup, err := svc.StartBackup(ctx, rule)

	assert.Nil(t, err)
	assert.Equal(t, rule.Name, backup.Rule)
	assert.Equal(t, ExecStatusStarted, backup.ExecStatus)
	assert.NotEqual(t, "", backup.ContainerId)
	assert.Equal(t, tempDirectory, backup.TempDirectory)
}

// endregion

// region Test: FinishBackup
func TestService_FinishBackup(t *testing.T) {
	repo := &backupRepositoryMock{}
	dockerClient := &dockerClientMock{}
	mountManager := &mountManagerMock{}
	transferManager := &transferManagerMock{}

	backup := Backup{
		Rule:          "some-rule",
		Id:            123456,
		ContainerId:   "some-container-id",
		TempDirectory: "/tmp/temp_dir",
		ExecStatus:    ExecStatusStarted,
	}

	ctx := context.Background()

	dockerClient.On("ContainerWait", ctx, backup.ContainerId).
		Return(int64(0), nil)

	transferManager.On("Transfer", backup).
		Return("/transfer/some_dir", nil)

	mountManager.On("Deallocate", backup.TempDirectory).
		Return(nil)

	repo.On("Update", ctx, Backup{
		Rule:            "some-rule",
		Id:              123456,
		ContainerId:     "some-container-id",
		TempDirectory:   "/tmp/temp_dir",
		BackupDirectory: "/transfer/some_dir", // this is updated
		ExecStatus:      ExecStatusSuccess,    // and this is too
	}).Return(nil)

	dockerClient.On("ContainerRemove", mock.Anything, backup.ContainerId, mock.Anything).Return(nil)

	svc := NewBackupService(discardLogger(), repo, dockerClient, mountManager, transferManager)

	resultBackup, err := svc.FinishBackup(ctx, backup)

	assert.Nil(t, err)
	assert.Equal(t, ExecStatusSuccess, resultBackup.ExecStatus)
}

// endregion
