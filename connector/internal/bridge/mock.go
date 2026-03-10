package bridge

import (
	"context"
	"fmt"
	"time"
)

type MockDriver struct {
	events      chan Event
	lastWritten string
	readerName  string
	stop        chan struct{}
}

func NewMockDriver(readerName string) *MockDriver {
	driver := &MockDriver{
		events:     make(chan Event, 32),
		readerName: readerName,
		stop:       make(chan struct{}),
	}

	go driver.loop()
	return driver
}

func (d *MockDriver) DriverName() string {
	return "mock"
}

func (d *MockDriver) Health(context.Context) map[string]any {
	return map[string]any{
		"status": "ok",
		"driver": d.DriverName(),
	}
}

func (d *MockDriver) ListReaders(context.Context) ([]Reader, error) {
	return []Reader{{
		Name:       d.readerName,
		Driver:     d.DriverName(),
		CardPresent: true,
	}}, nil
}

func (d *MockDriver) ConnectSession(context.Context, string) (*Session, error) {
	session := NewSession(d.readerName, d.DriverName())
	d.emit(Event{
		Type:      "reader.status",
		Status:    "ready",
		Reader:    &Reader{Name: d.readerName, Driver: d.DriverName(), CardPresent: true},
		SessionID: session.ID,
	})
	d.emit(Event{
		Type:      "card.present",
		Status:    "ready",
		Reader:    &Reader{Name: d.readerName, Driver: d.DriverName(), CardPresent: true},
		SessionID: session.ID,
		Payload:   map[string]any{"mode": "mock"},
	})
	return session, nil
}

func (d *MockDriver) ReadCard(context.Context, *Session, string) (*CardReadResult, error) {
	result := &CardReadResult{
		Reader:    d.readerName,
		Operation: "summary",
		UID:       "04AABBCCDDEE",
		ATR:       "3B8F8001804F0CA000000306030001000000006A",
		Details: map[string]string{
			"driver":      d.DriverName(),
			"lastWritten": d.lastWritten,
		},
	}
	d.emit(Event{
		Type:   "card.read.complete",
		Status: "ok",
		Reader: &Reader{Name: d.readerName, Driver: d.DriverName(), CardPresent: true},
		Payload: map[string]any{
			"uid": result.UID,
		},
	})
	return result, nil
}

func (d *MockDriver) WriteCard(_ context.Context, session *Session, request *WriteRequest) (*CardWriteResult, error) {
	d.lastWritten = string(request.EncodedPayload)
	result := &CardWriteResult{
		SessionID: session.ID,
		Reader:    d.readerName,
		Operation: request.Operation,
		Accepted:  true,
		Details: map[string]string{
			"mode":        "mock",
			"profile":     request.Profile,
			"mediaType":   request.MediaType,
			"payloadType": request.PayloadType,
			"payload":     d.lastWritten,
		},
	}
	d.emit(Event{
		Type:      "card.write.complete",
		Status:    "ok",
		Reader:    &Reader{Name: d.readerName, Driver: d.DriverName(), CardPresent: true},
		SessionID: session.ID,
		Payload: map[string]any{
			"operation":   request.Operation,
			"profile":     request.Profile,
			"payloadType": request.PayloadType,
		},
	})
	return result, nil
}

func (d *MockDriver) Events() <-chan Event {
	return d.events
}

func (d *MockDriver) Close() error {
	close(d.stop)
	return nil
}

func (d *MockDriver) emit(event Event) {
	event.At = time.Now().UTC().Format(time.RFC3339)
	select {
	case d.events <- event:
	default:
	}
}

func (d *MockDriver) loop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.emit(Event{
				Type:   "reader.status",
				Status: "ready",
				Reader: &Reader{Name: d.readerName, Driver: d.DriverName(), CardPresent: true},
				Payload: map[string]any{
					"heartbeat": fmt.Sprintf("%d", time.Now().Unix()),
				},
			})
		case <-d.stop:
			close(d.events)
			return
		}
	}
}