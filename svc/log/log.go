package log

import (
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func Init() {
	plainLogger, err := zap.NewDevelopment()
	if err != nil {

		panic(err)
	}

	logger = plainLogger.Sugar()
}

func SyncLogs() error {
	err := logger.Sync()
	if err != nil {
		return err
	}

	return nil
}

func Fatalf(msg string, rest ...interface{}) {
	logger.Fatalf(msg, rest...)
}

func Errorw(msg string, rest ...interface{}) {
	logger.Errorw(msg, rest...)
}

func Errorf(msg string, rest ...interface{}) {
	logger.Errorf(msg, rest...)
}

func Infow(msg string, rest ...interface{}) {
	logger.Infow(msg, rest...)
}
