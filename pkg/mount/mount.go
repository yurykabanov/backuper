package mount

import (
	"os"
	"path/filepath"

	"github.com/yurykabanov/backuper/pkg/util"
)

type Manager struct {
	base string
}

func New(base string) *Manager {
	return &Manager{
		base: base,
	}
}

func (m *Manager) Allocate() (string, error) {
	dir := filepath.Join(m.base, util.RandStringBytesMaskImprSrc(40))

	err := os.Mkdir(dir, os.ModeDir|os.ModePerm)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (m *Manager) Deallocate(dir string) error {
	return os.RemoveAll(dir)
}
