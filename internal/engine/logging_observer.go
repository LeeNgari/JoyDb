package engine

import "log/slog"

// LoggingObserver is a simple observer that logs all events
type LoggingObserver struct {
	logger *slog.Logger
}

func NewLoggingObserver() *LoggingObserver {
	return &LoggingObserver{
		logger: slog.Default(),
	}
}

func (lo *LoggingObserver) OnEvent(event Event) {
	lo.logger.Debug("query_lifecycle",
		"event", event.Type,
		"tx_id", event.TxID,
		"timestamp", event.Timestamp,
		"data", event.Data,
	)
}
