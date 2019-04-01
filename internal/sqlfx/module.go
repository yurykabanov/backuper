package sqlfx

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(SqliteConfigProvider),
	fx.Provide(OpenSqliteDatabase),
	fx.Provide(BackupsRepository),
	fx.Invoke(CloseSqliteDatabase),
)
