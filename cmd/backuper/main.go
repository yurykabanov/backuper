package main

import (
	"context"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/yurykabanov/backuper/pkg/domain"
	"github.com/yurykabanov/backuper/pkg/http/handler"
	"github.com/yurykabanov/backuper/pkg/http/middleware"
	"github.com/yurykabanov/backuper/pkg/mount"
	"github.com/yurykabanov/backuper/pkg/storage"
	"github.com/yurykabanov/backuper/pkg/transfer"
	"github.com/yurykabanov/backuper/pkg/util"

	"github.com/sirupsen/logrus"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	docker "github.com/docker/docker/client"
	"github.com/robfig/cron"

	"github.com/gorilla/mux"
)

var (
	Build   = "unknown"
	Version = "unknown"
)

const (
	EnvPrefix              = "backuper"
	DefaultConfigDirectory = "backuper"
	DefaultConfigFile      = "backuper"
)

const (
	ConfigLogLevel  = "log.level"
	ConfigLogFormat = "log.format"

	ConfigServerAddress      = "server.address"
	ConfigServerTimeoutRead  = "server.timeout.read"
	ConfigServerTimeoutWrite = "server.timeout.write"
	ConfigServerLogRequests  = "server.log.requests"

	ConfigDockerHost    = "docker.host"
	ConfigDockerVersion = "docker.version"

	ConfigMountTempDirectory = "mount.temp_directory"
)

var (
	DefaultConfigPaths = []string{
		".",
		"./config",
		path.Join("/etc", DefaultConfigDirectory),
	}
)

func LoadConfiguration() {
	// Config file flag
	pflag.StringP("config", "c", "", "Config file")

	pflag.String(ConfigLogLevel, "info", "Log level")
	pflag.String(ConfigLogFormat, "json", "Log output format")

	// NOTE: we don't have logger configured yet as we haven't read all sources of configuration
	// so we're using default logrus logger as fallback
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		logrus.WithError(err).Fatal("Couldn't bind flags")
	}

	// Read config from environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix(EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config from config file
	if configFile := viper.GetString("config"); configFile != "" {
		// If user do specify config file, then this file MUST exist and be valid
		// so missing file is a fatal error

		viper.SetConfigFile(configFile)

		if err := viper.ReadInConfig(); err != nil {
			logrus.WithError(err).Fatal("Couldn't read config file")
		}
	} else {
		// If user does not specify config file, then we'll still try to find appropriate config,
		// but missing file is not an error

		viper.SetConfigName(DefaultConfigFile)

		for _, dir := range DefaultConfigPaths {
			viper.AddConfigPath(dir)
		}

		if err := viper.ReadInConfig(); err != nil {
			logrus.WithError(err).Warn("Couldn't read config file")
		}
	}
}

func MustCreateLogger(logLevel, logFormat string) *logrus.Logger {
	// logrus logger is used anywhere throughout the app
	logrusLogger := logrus.StandardLogger()

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}

	logrusLogger.SetLevel(level)

	switch logFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		fallthrough
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	return logrusLogger
}

func DefaultLoggerAdapter(logger *logrus.Logger) *log.Logger {
	loggerWriter := logger.Writer()
	// NOTE: loggerWriter is never closed, but logger is supposed to live until application is closed, so this is fine

	return log.New(loggerWriter, "", 0)
}

func MustOpenMysql(logger logrus.FieldLogger) *sqlx.DB {
	dsn := "./db/sqlite3.db"

	logger.WithField("dsn", dsn).Debug("Connecting to DB with DSN")

	db, err := sqlx.Open("sqlite3", dsn)
	if err != nil {
		logger.WithError(err).Fatal("Unable to connect to DB")
	}

	db.MapperFunc(util.CamelToSnakeCase)

	driver, err := migratesqlite.WithInstance(db.DB, &migratesqlite.Config{})
	if err != nil {
		logger.WithError(err).Fatal("Unable to create instance of migrate")
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations/", "backuper", driver)
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		logger.WithError(err).Fatal("Unable to migrate DB")
	}

	return db
}

func MustConnectToDocker(logger logrus.FieldLogger) *docker.Client {
	host := viper.GetString(ConfigDockerHost)
	version := viper.GetString(ConfigDockerVersion)

	logger.WithField("host", host).Debug("Connecting to docker via")

	dc, err := docker.NewClient(host, version, nil, nil)
	if err != nil {
		logger.WithError(err).Fatal("Unable to create docker client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = dc.Ping(ctx)
	if err != nil {
		logger.WithError(err).Fatal("Unable to ping docker")
	}

	return dc
}

func MustLoadRules(logger logrus.FieldLogger) []domain.Rule {
	var rr []domain.Rule

	err := viper.UnmarshalKey("rules", &rr)
	if err != nil {
		logger.WithError(err).Fatal("unable to unmarshal rules")
	}

	return rr
}

func MustBuildHttpHandler(
	logger logrus.FieldLogger,
	router http.Handler,
) *http.Server {
	var h = router

	if viper.GetBool(ConfigServerLogRequests) {
		h = middleware.WithRequestLogging(router, logger)
	}

	h = middleware.WithRequestId(h, middleware.DefaultRequestIdProvider)

	return &http.Server{
		Addr:         viper.GetString(ConfigServerAddress),
		ReadTimeout:  viper.GetDuration(ConfigServerTimeoutRead),
		WriteTimeout: viper.GetDuration(ConfigServerTimeoutWrite),
		Handler:      h,
	}
}

func main() {
	LoadConfiguration()
	logger := MustCreateLogger(viper.GetString(ConfigLogLevel), viper.GetString(ConfigLogFormat))

	logger.WithFields(logrus.Fields{
		"build":   Build,
		"version": Version,
	}).Info("Application is starting...")

	dockerClient := MustConnectToDocker(logger)

	db := MustOpenMysql(logger)
	defer db.Close()

	backupRepository := storage.NewBackupRepository(db)

	mountManager := mount.New(viper.GetString(ConfigMountTempDirectory))
	transferManager := transfer.New()

	backupService := domain.NewBackupService(logger, backupRepository, dockerClient, mountManager, transferManager)

	rules := MustLoadRules(logger)
	backupManager := domain.NewBackupManager(logger, rules, backupService, backupRepository, cron.New())

	backupsHandler := handler.NewBackupMetricHandler(logger, rules, backupRepository)

	router := mux.NewRouter()
	router.Handle("/metrics/backups", backupsHandler)

	server := MustBuildHttpHandler(logger, router)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).WithField("addr", "0.0.0.0:8000").Fatal("Unable to run http server")
		}
		wg.Done()
	}()

	go func() {
		backupManager.Run()
		wg.Done()
	}()

	wg.Wait()
}
