package clients

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

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

func InitLog(fileName string) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("falha ao obter o caminho do arquivo atual")
		os.Exit(1)
	}

	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	logDir := filepath.Join(projectRoot, "log")
	logFilePath := filepath.Join(logDir, fileName)


	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("falha ao criar o diretório de logs: %v\n", err)
		os.Exit(1)
	}

	lumberjackLogger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    100,  
		MaxBackups: 3,    
		MaxAge:     28,   
		Compress:   true, 
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}

	fileLevelWriter := &levelWriter{
		writer: zerolog.MultiLevelWriter(lumberjackLogger),
		level:  zerolog.InfoLevel,
	}

	multi := zerolog.MultiLevelWriter(consoleWriter, fileLevelWriter)
	logger := zerolog.New(multi).With().Timestamp().Logger()

	logger = logger.Level(zerolog.DebugLevel)

	log.Logger = logger

	log.Info().Msg("Logger configurado com sucesso")
	log.Info().Str("log_file", logFilePath).Msg("Logs serão gravados neste arquivo")
}
