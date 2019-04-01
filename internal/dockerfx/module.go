package dockerfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(DockerConnectionConfigProvider),
	fx.Provide(DockerClient),
	fx.Invoke(CloseDockerClient),
)
