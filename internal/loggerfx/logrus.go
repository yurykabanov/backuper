package loggerfx

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	ConfigLogLevel  = "log.level"
	ConfigLogFormat = "log.format"
)

var logger *logrus.Logger

func init() {
	logger = logrus.StandardLogger()
	logger.SetFormatter(&logrus.JSONFormatter{})
}

func Logger() *logrus.Logger {
	return logger
}

func ConfigureLogger(logger *logrus.Logger, v *viper.Viper) {
	logLevel := v.GetString(ConfigLogLevel)
	logFormat := v.GetString(ConfigLogFormat)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}

	logger.SetLevel(level)

	switch logFormat {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	default:
		fallthrough
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{})
	}
}
