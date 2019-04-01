package sqlfx

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/fx"

	"github.com/yurykabanov/backuper/pkg/util"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteConfig struct {
	DSN            string
	DatabaseName   string
	MigrationsPath string
}

func SqliteConfigProvider(v *viper.Viper) (*SqliteConfig, error) {
	config := &SqliteConfig{
		DSN:            "./db/sqlite3.db?parseTime=true",
		DatabaseName:   "backuper",
		MigrationsPath: "file://migrations/",
	}

	return config, nil
}

func OpenSqliteDatabase(config *SqliteConfig, logger *logrus.Logger) (*sqlx.DB, error) {
	logger.WithField("dsn", config.DSN).Debug("Connecting to DB with DSN")

	db, err := sqlx.Open("sqlite3", config.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to connect to DB")
	}

	db.MapperFunc(util.CamelToSnakeCase)

	driver, err := migratesqlite.WithInstance(db.DB, &migratesqlite.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create instance of migrate")
	}

	m, err := migrate.NewWithDatabaseInstance(config.MigrationsPath, config.DatabaseName, driver)
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return nil, errors.Wrap(err, "Unable to migrate DB")
	}

	return db, nil
}

func CloseSqliteDatabase(lc fx.Lifecycle, db *sqlx.DB) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})
}
