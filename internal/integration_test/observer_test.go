package integration

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/engine"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// TestQueryLifecycleEvents verifies that all expected events are emitted during query execution
func TestQueryLifecycleEvents(t *testing.T) {
	// Setup test database
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// Create engine with mock observer
	storageEng := storageEngine.NewJSONEngine()
	registry := manager.NewRegistry("../../databases", storageEng)
	eng := engine.New(db, registry)
	observer := &MockObserver{}
	eng.AddObserver(observer)

	// Execute a SELECT query
	sql := "SELECT * FROM users"
	_, err := eng.Execute(sql)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	// Verify events were emitted
	expectedEventTypes := []engine.EventType{
		engine.EventLexStart,
		engine.EventLexEnd,
		engine.EventParseStart,
		engine.EventParseEnd,
		engine.EventPlanStart,
		engine.EventPlanEnd,
		engine.EventExecStart,
		engine.EventExecEnd,
	}

	if len(observer.Events) != len(expectedEventTypes) {
		t.Errorf("Expected %d events, got %d", len(expectedEventTypes), len(observer.Events))
		for i, event := range observer.Events {
			t.Logf("Event %d: %s", i, event.Type)
		}
		return
	}

	// Verify event order and types
	for i, expectedType := range expectedEventTypes {
		if observer.Events[i].Type != expectedType {
			t.Errorf("Event %d: Expected %s, got %s", i, expectedType, observer.Events[i].Type)
		}
	}

	// Verify all events have the same TxID
	txID := observer.Events[0].TxID
	for i, event := range observer.Events {
		if event.TxID != txID {
			t.Errorf("Event %d: TxID mismatch. Expected %s, got %s", i, txID, event.TxID)
		}
	}

	// Verify timestamps are in chronological order
	for i := 1; i < len(observer.Events); i++ {
		if observer.Events[i].Timestamp.Before(observer.Events[i-1].Timestamp) {
			t.Errorf("Event %d timestamp is before event %d", i, i-1)
		}
	}
}

// TestEventDataContent verifies that event data contains expected values
func TestEventDataContent(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewJSONEngine()
	registry := manager.NewRegistry("../../databases", storageEng)
	eng := engine.New(db, registry)
	observer := &MockObserver{}
	eng.AddObserver(observer)

	sql := "SELECT * FROM users"
	_, err := eng.Execute(sql)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	// Check LexStart event contains SQL
	lexStartEvent := observer.Events[0]
	if lexStartEvent.Type != engine.EventLexStart {
		t.Fatalf("First event should be EventLexStart")
	}
	if lexStartEvent.Data != sql {
		t.Errorf("EventLexStart data should contain SQL. Got: %v", lexStartEvent.Data)
	}

	// Check LexEnd event contains token count
	lexEndEvent := observer.Events[1]
	if lexEndEvent.Type != engine.EventLexEnd {
		t.Fatalf("Second event should be EventLexEnd")
	}
	if tokenCount, ok := lexEndEvent.Data.(int); !ok || tokenCount <= 0 {
		t.Errorf("EventLexEnd data should contain positive token count. Got: %v", lexEndEvent.Data)
	}

	// Check ExecEnd event contains result data
	execEndEvent := observer.Events[len(observer.Events)-1]
	if execEndEvent.Type != engine.EventExecEnd {
		t.Fatalf("Last event should be EventExecEnd")
	}
	if resultData, ok := execEndEvent.Data.(map[string]interface{}); ok {
		if _, hasRowsReturned := resultData["rows_returned"]; !hasRowsReturned {
			t.Error("EventExecEnd data should contain rows_returned")
		}
		if _, hasRowsAffected := resultData["rows_affected"]; !hasRowsAffected {
			t.Error("EventExecEnd data should contain rows_affected")
		}
	} else {
		t.Errorf("EventExecEnd data should be a map. Got: %T", execEndEvent.Data)
	}
}

// TestMultipleQueries verifies that each query gets its own TxID
func TestMultipleQueries(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewJSONEngine()
	registry := manager.NewRegistry("../../databases", storageEng)
	eng := engine.New(db, registry)
	observer := &MockObserver{}
	eng.AddObserver(observer)

	// Execute first query
	_, err := eng.Execute("SELECT * FROM users")
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}

	firstQueryEventCount := len(observer.Events)
	firstTxID := observer.Events[0].TxID

	// Execute second query (same table, different query)
	_, err = eng.Execute("SELECT id FROM users")
	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	// Verify second query also emitted events
	if len(observer.Events) != firstQueryEventCount*2 {
		t.Errorf("Expected %d total events, got %d", firstQueryEventCount*2, len(observer.Events))
	}

	// Verify second query has different TxID
	secondTxID := observer.Events[firstQueryEventCount].TxID
	if firstTxID == secondTxID {
		t.Error("Different queries should have different TxIDs")
	}
}

// MockObserver for testing
type MockObserver struct {
	Events []engine.Event
}

func (m *MockObserver) OnEvent(event engine.Event) {
	m.Events = append(m.Events, event)
}
