package engine

import (
	"testing"
)

type MockObserver struct {
	Events []Event
}

func (m *MockObserver) OnEvent(event Event) {
	m.Events = append(m.Events, event)
}

func TestAddObserver(t *testing.T) {
	eng := New(nil, nil)
	observer := &MockObserver{}

	eng.AddObserver(observer)

	if len(eng.observers) != 1 {
		t.Errorf("Expected 1 observer, got %d", len(eng.observers))
	}
}

func TestRemoveObserver(t *testing.T) {
	eng := New(nil, nil)
	observer := &MockObserver{}

	eng.AddObserver(observer)
	eng.RemoveObserver(observer)

	if len(eng.observers) != 0 {
		t.Errorf("Expected 0 observers, got %d", len(eng.observers))
	}
}

func TestNotifyWithNoObservers(t *testing.T) {
	eng := New(nil, nil)

	eng.notify(Event{Type: EventLexStart, TxID: 123})
}

func TestNotifyWithMultipleObservers(t *testing.T) {
	eng := New(nil, nil)
	observer1 := &MockObserver{}
	observer2 := &MockObserver{}

	eng.AddObserver(observer1)
	eng.AddObserver(observer2)

	testEvent := Event{Type: EventLexStart, TxID: 123, Data: "SELECT * FROM users"}
	eng.notify(testEvent)

	if len(observer1.Events) != 1 {
		t.Errorf("Observer1: Expected 1 event, got %d", len(observer1.Events))
	}
	if len(observer2.Events) != 1 {
		t.Errorf("Observer2: Expected 1 event, got %d", len(observer2.Events))
	}

	if observer1.Events[0].Type != EventLexStart {
		t.Errorf("Observer1: Expected EventLexStart, got %v", observer1.Events[0].Type)
	}
	if observer2.Events[0].Type != EventLexStart {
		t.Errorf("Observer2: Expected EventLexStart, got %v", observer2.Events[0].Type)
	}
}

func TestEventTimestamp(t *testing.T) {
	eng := New(nil, nil)
	observer := &MockObserver{}
	eng.AddObserver(observer)

	eng.notify(Event{Type: EventLexStart, TxID: 123})

	if observer.Events[0].Timestamp.IsZero() {
		t.Error("Expected timestamp to be set, got zero value")
	}
}
