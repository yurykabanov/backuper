package metricsfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(HttpServerConfigProvider),
	fx.Provide(HttpServer),
	fx.Provide(HttpRouter),
	fx.Provide(Listener),
	fx.Invoke(RunServer),

	fx.Provide(LatestBackupMetricHandler),
	fx.Invoke(RegisterLatestBackupMetricHandler),
)
