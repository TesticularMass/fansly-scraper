package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/agnosto/fansly-scraper/config"
)

const (
	maxLogSize    = 5 * 1024 * 1024 // 5MB
	maxLogBackups = 5
)

var (
	// Logger defaults to a discard logger so code paths that run before
	// InitLogger (config wizard, tests) can't hit a nil logger.
	Logger = log.New(io.Discard, "", log.LstdFlags)
)

func InitLogger(cfg *config.Config) error {
	logDir := filepath.Join(cfg.Options.SaveLocation, ".logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "fansly-scraper.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	Logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

	go rotateLogFile(logFile)

	return nil
}

func rotateLogFile(logFile string) {
	for {
		time.Sleep(1 * time.Hour)

		file, err := os.Stat(logFile)
		if err != nil {
			Logger.Printf("Error checking log file: %v", err)
			continue
		}

		if file.Size() < maxLogSize {
			continue
		}

		Logger.Printf("Rotating log file")

		for i := maxLogBackups - 1; i > 0; i-- {
			oldFile := fmt.Sprintf("%s.%d", logFile, i)
			newFile := fmt.Sprintf("%s.%d", logFile, i+1)
			os.Rename(oldFile, newFile)
		}

		// Close before renaming: Windows refuses to rename an open file, and
		// the old code then truncated the still-live log, losing its content.
		if f, ok := Logger.Writer().(*os.File); ok {
			f.Close()
		}

		if err := os.Rename(logFile, logFile+".1"); err != nil {
			Logger.Printf("Error rotating log file: %v", err)
		}

		// Open a new log file (append, not truncate, in case the rename failed)
		newFile, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			continue
		}

		Logger.SetOutput(newFile)
	}
}
