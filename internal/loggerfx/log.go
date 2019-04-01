package loggerfx

import (
	"log"

	"github.com/sirupsen/logrus"
)

func DefaultLoggerAdapter(logger *logrus.Logger) *log.Logger {
	loggerWriter := logger.Writer()

	return log.New(loggerWriter, "", 0)
}
