package engine

import "log/slog"

// LoggingObserver is a simple observer that logs all events using structured logging
type LoggingObserver struct {
	logger *slog.Logger
}

// NewLoggingObserver creates a new logging observer
func NewLoggingObserver() *LoggingObserver {
	return &LoggingObserver{
		logger: slog.Default(),
	}
}

// OnEvent implements the Observer interface
// It logs each event with structured fields for easy filtering and analysis
func (lo *LoggingObserver) OnEvent(event Event) {
	lo.logger.Info("query_lifecycle",
		"event", event.Type,
		"tx_id", event.TxID,
		"timestamp", event.Timestamp,
		"data", event.Data,
	)
}
