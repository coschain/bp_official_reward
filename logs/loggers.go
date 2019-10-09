package logs

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"bp_official_reward/config"
)

// const
const (
	PanicLevel = "panic"
	FatalLevel = "fatal"
	ErrorLevel = "error"
	WarnLevel  = "warn"
	InfoLevel  = "info"
	DebugLevel = "debug"
)

type emptyWriter struct{}

func (ew emptyWriter) Write(p []byte) (int, error) {
	return 0, nil
}

func convertLevel(level string) logrus.Level {
	switch level {
	case PanicLevel:
		return logrus.PanicLevel
	case FatalLevel:
		return logrus.FatalLevel
	case ErrorLevel:
		return logrus.ErrorLevel
	case WarnLevel:
		return logrus.WarnLevel
	case InfoLevel:
		return logrus.InfoLevel
	case DebugLevel:
		return logrus.DebugLevel
	default:
		return logrus.InfoLevel
	}
}
var logger *logrus.Logger

func StartLogService() (*logrus.Logger,error) {
	if logger == nil {
		path,err := resolveLogPath("logs")
		if err != nil {
			fmt.Printf("Fail to get log output path, the error is %v", err)
			return nil,err
		}
		logger = initLog(path, DebugLevel, 86400 * 120)
	}
	return logger,nil
}

func GetLogger() *logrus.Logger {
	return logger
}

// Init loggers
func initLog(path string, level string, age uint32) *logrus.Logger {
	fileHooker := NewFileRotateHooker(path, age)
	var clog *logrus.Logger

	clog = logrus.New()
	LoadFunctionHooker(clog)
	clog.Hooks.Add(fileHooker)
	clog.Out = os.Stdout
	clog.Formatter = &TextFormatter{
		ForceColors:     true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	}
	clog.Level = convertLevel(level)

	return clog
}

func resolveLogPath(path string) (string,error) {
	if filepath.IsAbs(path) {
		return path,nil
	}
	dir,err := getLogDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, path),nil
}

func getLogDirPath() (string,error)  {
	//place the log directory under the same path as the project directory
	logDir,err := filepath.Abs(config.GetLogOutputPath())
	if err != nil {
		return "", nil
	}
	return filepath.Join(logDir, "bp_reward"), nil
}