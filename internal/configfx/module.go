package configfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(PFlags),
	fx.Provide(ViperProvider),
)
