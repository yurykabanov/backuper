package loggerfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(Logger),
	fx.Provide(DefaultLoggerAdapter),
	fx.Invoke(ConfigureLogger),
)
