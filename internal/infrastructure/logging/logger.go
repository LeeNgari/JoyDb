package logging

import (
	"context"
	"log/slog"
	"os"
	"time"

	slogseq "github.com/sokkalf/slog-seq"
)

// multiHandler forwards log records to multiple handlers
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Enable if any handler is enabled for this level
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// SetupLogger initializes the global logger and returns a cleanup function
func SetupLogger() (*slog.Logger, func()) {
	// Console handler
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	// Seq handler
	_, seqHandler := slogseq.NewLogger(
		"http://localhost:5341",
		slogseq.WithBatchSize(1),
		slogseq.WithFlushInterval(500*time.Millisecond),
		slogseq.WithHandlerOptions(&slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}),
	)


	// If Seq is not available, use console only
	if seqHandler == nil {
		console := slog.New(consoleHandler)
		console.Warn("Seq server unavailable - logging to console only",
			slog.String("seq_url", "http://localhost:5341"),
			slog.String("fallback", "console"),
		)
		return console, func() {}
	}

	// Combine both handlers
	multi := &multiHandler{
		handlers: []slog.Handler{consoleHandler, seqHandler},
	}

	logger := slog.New(multi)

	closeFn := func() {
		_ = seqHandler.Close()
	}

	return logger, closeFn
}
