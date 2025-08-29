package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func Init(level string) {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})

	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
}

func Debug(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	log.Debugf(format, args...)
}

func Info(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	log.Infof(format, args...)
}

func Warn(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	log.Warnf(format, args...)
}

func Error(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	log.Errorf(format, args...)
}

func Fatal(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf(format+"\n", args...)
		os.Exit(1)
	}
	log.Fatalf(format, args...)
}

func WithField(key string, value interface{}) *logrus.Entry {
	if log == nil {
		return logrus.NewEntry(logrus.StandardLogger()).WithField(key, value)
	}
	return log.WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	if log == nil {
		return logrus.NewEntry(logrus.StandardLogger()).WithFields(fields)
	}
	return log.WithFields(fields)
}