package engine

import "time"

// EventType represents different lifecycle phases in query execution
type EventType string

const (
	EventLexStart   EventType = "lex_start"
	EventLexEnd     EventType = "lex_end"
	EventParseStart EventType = "parse_start"
	EventParseEnd   EventType = "parse_end"
	EventPlanStart  EventType = "plan_start"
	EventPlanEnd    EventType = "plan_end"
	EventExecStart  EventType = "exec_start"
	EventExecEnd    EventType = "exec_end"
)

// Event represents a lifecycle event in query execution
type Event struct {
	Type      EventType   // Type of event
	TxID      string      // Transaction ID for tracing
	Timestamp time.Time   // When the event occurred
	Data      interface{} // Phase-specific data (e.g., SQL, token count, AST, result)
}

// Observer interface for event subscribers
// Observers receive events at major execution phases
type Observer interface {
	OnEvent(event Event)
}
