package dockerfx

import (
	"context"
	"time"

	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

const (
	ConfigDockerHost    = "docker.host"
	ConfigDockerVersion = "docker.version"
)

type DockerConnectionConfig struct {
	Host    string
	Version string
}

func DockerConnectionConfigProvider(v *viper.Viper) (*DockerConnectionConfig, error) {
	return &DockerConnectionConfig{
		Host:    v.GetString(ConfigDockerHost),
		Version: v.GetString(ConfigDockerVersion),
	}, nil
}

func DockerClient(config *DockerConnectionConfig, logger *logrus.Logger) (*docker.Client, error) {
	logger.WithField("host", config.Host).Debug("Connecting to docker via")

	client, err := docker.NewClient(config.Host, config.Version, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create docker client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Ping(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to ping docker")
	}

	return client, nil
}

func CloseDockerClient(lc fx.Lifecycle, client *docker.Client) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
}
