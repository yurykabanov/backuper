package transfer

import (
	"errors"

	"github.com/yurykabanov/backuper/pkg/domain"
)

var (
	ErrMountDoesNotExist = errors.New("requested storage doesn't exist")
)

type Manager struct {
	mounts map[string]domain.TransferManager
}

func NewManager(mounts map[string]domain.TransferManager) *Manager {
	return &Manager{
		mounts: mounts,
	}
}

func (m *Manager) Transfer(backup domain.Backup) (string, error) {
	if mount, ok := m.mounts[backup.StorageName]; ok {
		return mount.Transfer(backup)
	}
	return "", ErrMountDoesNotExist
}

func (m *Manager) Remove(backup domain.Backup) error {
	if mount, ok := m.mounts[backup.StorageName]; ok {
		return mount.Remove(backup)
	}
	return ErrMountDoesNotExist
}
