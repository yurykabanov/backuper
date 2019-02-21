package domain

import (
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// region mapBackupRepository
type mapBackupRepository struct {
	lastId  int64
	backups map[int64]Backup
}

func newMapBackupRepository() *mapBackupRepository {
	return &mapBackupRepository{
		backups: make(map[int64]Backup),
	}
}

func (r *mapBackupRepository) Create(backup Backup) (Backup, error) {
	r.lastId++
	backup.Id = r.lastId

	r.backups[backup.Id] = backup

	return backup, nil
}

func (r *mapBackupRepository) Update(backup Backup) error {
	r.backups[backup.Id] = backup

	return nil
}

func (r *mapBackupRepository) FindAllUnfinished() ([]Backup, error) {
	// TODO
	panic("implement me")
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

func (m *transferManagerMock) Transfer(backup Backup) error {
	args := m.Called(backup)
	return args.Error(0)
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

// region Test: pullImage
func TestService_pullImage_Success(t *testing.T) {
	dockerClient := &dockerClientMock{}

	dockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).
		Return(ioutil.NopCloser(strings.NewReader("some response")), nil)

	svc := NewBackupService(newMapBackupRepository(), dockerClient, nil, nil)

	err := svc.pullImage(context.Background(), &namedReference{})

	assert.Nil(t, err)
}

func TestService_pullImage_Failure(t *testing.T) {
	dockerClient := &dockerClientMock{}

	dockerClient.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).
		Return(io.ReadCloser(nil), context.DeadlineExceeded)

	svc := NewBackupService(newMapBackupRepository(), dockerClient, nil, nil)

	err := svc.pullImage(context.Background(), &namedReference{})

	assert.Equal(t, context.DeadlineExceeded, err)
}

// endregion

// region Test: StartBackup
func TestService_StartBackup(t *testing.T) {
	dockerClient := &dockerClientMock{}
	mountManager := &mountManagerMock{}
	transferManager := &transferManagerMock{}

	rule := Rule{
		Name:            "some_rule",
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

	svc := NewBackupService(newMapBackupRepository(), dockerClient, mountManager, transferManager)

	backup, err := svc.StartBackup(ctx, rule)

	assert.Nil(t, err)
	assert.Equal(t, rule, backup.Rule)
	assert.Equal(t, ExecStatusStarted, backup.ExecStatus)
	assert.NotEqual(t, "", backup.ContainerId)
	assert.Equal(t, tempDirectory, backup.TempDirectory)
}

// endregion

// region Test: FinishBackup
func TestService_FinishBackup(t *testing.T) {
	dockerClient := &dockerClientMock{}
	mountManager := &mountManagerMock{}
	transferManager := &transferManagerMock{}

	backup := Backup{
		Rule: Rule{
			Name:            "some_rule",
			Image:           "whatever/image:1.2.3",
			TargetDirectory: "/tmp/whatever/",
			Command:         []string{"echo", "123", ">", "$BACKUP_TARGET_DIRECTORY/backup.dat"},
		},
		Id:            "some-id",
		ContainerId:   "some-container-id",
		TempDirectory: "/tmp/temp_dir",
		ExecStatus:    ExecStatusStarted,
	}

	ctx := context.Background()

	dockerClient.On("ContainerWait", ctx, backup.ContainerId).
		Return(int64(0), nil)

	transferManager.On("Transfer", backup).
		Return(nil)

	mountManager.On("Deallocate", backup.TempDirectory).
		Return(nil)

	svc := NewBackupService(newMapBackupRepository(), dockerClient, mountManager, transferManager)

	backup, err := svc.FinishBackup(ctx, backup)

	assert.Nil(t, err)
	assert.Equal(t, ExecStatusSuccess, backup.ExecStatus)
}

// endregion
