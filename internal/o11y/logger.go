package o11y

import (
	"log/slog"
	"os"

	"github.com/charmbracelet/log"
)

func NewLogger() *slog.Logger {
	handler := log.New(os.Stderr)
	return slog.New(handler)
}
