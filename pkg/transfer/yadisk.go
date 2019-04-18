package transfer

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/yurykabanov/go-yandex-disk"

	"github.com/yurykabanov/backuper/pkg/domain"
)

type YaDiskMount struct {
	client *yadisk.Client
	root   string
}

func NewYaDiskMount(client *yadisk.Client, root string) *YaDiskMount {
	return &YaDiskMount{
		client: client,
		root:   root,
	}
}

func (m *YaDiskMount) Transfer(backup domain.Backup) (string, error) {
	name := fmt.Sprintf("%s_%s.zip", backup.Rule, backup.CreatedAt.UTC().Format("2006-01-02_15-04-05"))
	target := path.Join(m.root, name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	link, err := m.client.RequestUploadLink(ctx, target, false)
	if err != nil {
		return "", err
	}
	cancel()

	f, err := os.Open(backup.TempBackupFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = m.client.Upload(context.TODO(), link, f)
	if err != nil {
		return "", err
	}

	return target, nil
}

func (m *YaDiskMount) Remove(backup domain.Backup) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _, err := m.client.Delete(ctx, backup.BackupFile, true)
	return err
}
