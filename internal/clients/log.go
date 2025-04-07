package clients

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

type levelWriter struct {
	writer io.Writer
	level  zerolog.Level
}

func (lw *levelWriter) Write(p []byte) (n int, err error) {
	return lw.writer.Write(p)
}

func (lw *levelWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level >= lw.level {
		return lw.writer.Write(p)
	}
	return len(p), nil
}

func InitLog(fileName string, basePath string, silent ...bool) (lumberjackLogger *lumberjack.Logger, logFilePath string) {
	logDir := filepath.Join(basePath, "log")
	logFilePath = filepath.Join(logDir, fileName)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("falha ao criar o diretório de logs: %v\n", err)
		os.Exit(1)
	}

	lumberjackLogger = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Writer para o arquivo, com filtro de nível
	fileLevelWriter := &levelWriter{
		writer: zerolog.MultiLevelWriter(lumberjackLogger),
		level:  zerolog.InfoLevel,
	}

	var multi zerolog.LevelWriter
	if len(silent) > 0 && silent[0] {
		multi = fileLevelWriter
	} else {
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
		multi = zerolog.MultiLevelWriter(consoleWriter, fileLevelWriter)
	}

	logger := zerolog.New(multi).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = logger

	log.Info().Msg("Logger configurado com sucesso")
	log.Info().Str("log_file", logFilePath).Msg("Logs serão gravados neste arquivo")

	return lumberjackLogger, logFilePath
}
