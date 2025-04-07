package clients_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThalysSilva/ingestor-consumo/internal/clients"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestLog(t *testing.T) {

	t.Run("WriteLogManually", func(t *testing.T) {
		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "log")
		logFilePath := filepath.Join(logDir, "test.log")

		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			t.Fatalf("erro ao criar diretório de log: %v", err)
		}

		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   false,
		}

		message := []byte("mensagem de teste\n")
		_, err = lumberjackLogger.Write(message)
		if err != nil {
			t.Fatalf("erro ao escrever no logger: %v", err)
		}

		_ = lumberjackLogger.Close()

		data, err := os.ReadFile(logFilePath)
		if err != nil {
			t.Fatalf("erro ao ler o log: %v", err)
		}

		if !bytes.Contains(data, message) {
			t.Errorf("log não contém a mensagem esperada. Conteúdo:\n%s", string(data))
		}
	})

	t.Run("CreateLogFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, logFilePath := clients.InitLog("test.log", tmpDir)
		defer logger.Close()

		log.Info().Msg("forçando criação do arquivo")

		found := false
		for range 10 {
			if _, err := os.Stat(logFilePath); err == nil {
				found = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !found {
			t.Errorf("arquivo de log não foi criado: %s", logFilePath)
		}
	})

	t.Run("WriteLogMessage", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, logFilePath := clients.InitLog("test.log", tmpDir, true)
		defer logger.Close()

		log.Info().Msg("mensagem de teste")

		var data []byte
		var err error
		found := false
		for range 10 {
			data, err = os.ReadFile(logFilePath)
			if err == nil && bytes.Contains(data, []byte("mensagem de teste")) {
				found = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if err != nil {
			t.Fatalf("erro ao ler o log: %v", err)
		}

		if !found {
			t.Errorf("log não contém a mensagem esperada. Conteúdo:\n%s", string(data))
		}
	})
	t.Run("WriteDebugNotLoggedToFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		logger, logFilePath := clients.InitLog("test.log", tmpDir, true)
		defer logger.Close()

		log.Debug().Msg("esta mensagem de debug não deve ir para o arquivo")

		time.Sleep(200 * time.Millisecond)

		data, err := os.ReadFile(logFilePath)
		if err != nil {
			t.Fatalf("erro ao ler o log: %v", err)
		}

		if bytes.Contains(data, []byte("esta mensagem de debug")) {
			t.Errorf("mensagem de debug foi salva no log, mas não deveria. Conteúdo:\n%s", string(data))
		}
	})
}
