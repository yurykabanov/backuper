package main

import (
	"time"

	"go.uber.org/fx"

	"github.com/yurykabanov/backuper/internal/configfx"
	"github.com/yurykabanov/backuper/internal/dockerfx"
	"github.com/yurykabanov/backuper/internal/domainfx"
	"github.com/yurykabanov/backuper/internal/loggerfx"
	"github.com/yurykabanov/backuper/internal/metricsfx"
	"github.com/yurykabanov/backuper/internal/sqlfx"
)

func main() {
	logger := loggerfx.Logger()

	app := fx.New(
		fx.StartTimeout(15*time.Second),
		fx.StopTimeout(15*time.Second),

		fx.Logger(logger),

		loggerfx.Module,
		configfx.Module,
		sqlfx.Module,
		dockerfx.Module,
		metricsfx.Module,
		domainfx.Module,
	)

	app.Run()
}
