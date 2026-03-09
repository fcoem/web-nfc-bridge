//go:build darwin

package bridge

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ebfe/scard"
)

type PCSCDarwinDriver struct {
	ctx             *scard.Context
	events          chan Event
	stop            chan struct{}
	opMu            sync.Mutex
	lastReaderSet   string
	lastCardPresent bool
}

func NewPCSCDriver() (*PCSCDarwinDriver, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, err
	}

	driver := &PCSCDarwinDriver{
		ctx:    ctx,
		events: make(chan Event, 32),
		stop:   make(chan struct{}),
	}

	go driver.monitor()
	return driver, nil
}

func (d *PCSCDarwinDriver) DriverName() string {
	return "pcsc"
}

func (d *PCSCDarwinDriver) Health(context.Context) map[string]any {
	d.opMu.Lock()
	defer d.opMu.Unlock()

	valid, err := d.ctx.IsValid()
	status := "ok"
	if err != nil || !valid {
		status = "degraded"
	}
	return map[string]any{
		"status": status,
		"driver": d.DriverName(),
	}
}

func (d *PCSCDarwinDriver) ListReaders(context.Context) ([]Reader, error) {
	d.opMu.Lock()
	defer d.opMu.Unlock()
	return d.listReadersLocked()
}

func (d *PCSCDarwinDriver) listReadersLocked() ([]Reader, error) {
	readers, err := d.ctx.ListReaders()
	if err != nil {
		if errors.Is(err, scard.ErrNoReadersAvailable) {
			return []Reader{}, nil
		}
		return nil, err
	}

	items := make([]Reader, 0, len(readers))
	for _, name := range readers {
		items = append(items, Reader{
			Name:       name,
			Driver:     d.DriverName(),
			CardPreset: false,
		})
	}
	return items, nil
}

func (d *PCSCDarwinDriver) ConnectSession(_ context.Context, readerName string) (*Session, error) {
	d.opMu.Lock()
	defer d.opMu.Unlock()

	resolvedReader, err := d.resolveReaderLocked(readerName)
	if err != nil {
		return nil, err
	}
	if err := d.ensureCardPresentLocked(resolvedReader); err != nil {
		return nil, err
	}

	card, err := d.ctx.Connect(resolvedReader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return nil, err
	}
	defer card.Disconnect(scard.LeaveCard)

	session := NewSession(resolvedReader, d.DriverName())
	d.emit(Event{
		Type:      "reader.status",
		Status:    "ready",
		Reader:    &Reader{Name: resolvedReader, Driver: d.DriverName(), CardPreset: true},
		SessionID: session.ID,
	})
	return session, nil
}

func (d *PCSCDarwinDriver) ReadCard(_ context.Context, session *Session, operation string) (*CardReadResult, error) {
	d.opMu.Lock()
	defer d.opMu.Unlock()
	if err := d.ensureCardPresentLocked(session.ReaderName); err != nil {
		return nil, err
	}

	card, err := d.ctx.Connect(session.ReaderName, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return nil, err
	}
	defer card.Disconnect(scard.LeaveCard)

	status, err := card.Status()
	if err != nil {
		return nil, err
	}

	uidResponse, err := card.Transmit([]byte{0xFF, 0xCA, 0x00, 0x00, 0x00})
	if err != nil {
		return nil, err
	}

	uid := parseUID(uidResponse)
	result := &CardReadResult{
		SessionID: session.ID,
		Reader:    session.ReaderName,
		Operation: operation,
		UID:       uid,
		ATR:       strings.ToUpper(hex.EncodeToString(status.Atr)),
		Details: map[string]string{
			"driver":   d.DriverName(),
			"protocol": fmt.Sprintf("%d", status.ActiveProtocol),
		},
	}

	capability, err := readType2Capability(card)
	if err == nil {
		mediaType, payload, readErr := readType2NDEF(card, capability)
		if readErr == nil {
			result.MediaType = mediaType
			result.Payload = payload
		}
	}

	d.emit(Event{
		Type:      "card.read.complete",
		Status:    "ok",
		Reader:    &Reader{Name: session.ReaderName, Driver: d.DriverName(), CardPreset: true},
		SessionID: session.ID,
		Payload: map[string]any{
			"uid":       uid,
			"mediaType": result.MediaType,
			"payload":   result.Payload,
		},
	})

	return result, nil
}

func (d *PCSCDarwinDriver) WriteCard(_ context.Context, session *Session, request *WriteRequest) (*CardWriteResult, error) {
	d.opMu.Lock()
	defer d.opMu.Unlock()
	if err := d.ensureCardPresentLocked(session.ReaderName); err != nil {
		return nil, err
	}

	card, err := d.ctx.Connect(session.ReaderName, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return nil, err
	}
	defer func() {
		if card != nil {
			_ = card.Disconnect(scard.ResetCard)
		}
	}()

	capability, err := readType2Capability(card)
	if err != nil {
		return &CardWriteResult{
			SessionID: session.ID,
			Reader:    session.ReaderName,
			Operation: request.Operation,
			Accepted:  false,
			Details: map[string]string{
				"driver":      d.DriverName(),
				"profile":     request.Profile,
				"mediaType":   request.MediaType,
				"payloadType": request.PayloadType,
				"reason":      err.Error(),
			},
		}, nil
	}
	if capability.ReadOnly {
		return &CardWriteResult{
			SessionID: session.ID,
			Reader:    session.ReaderName,
			Operation: request.Operation,
			Accepted:  false,
			Details: map[string]string{
				"driver":      d.DriverName(),
				"profile":     request.Profile,
				"mediaType":   request.MediaType,
				"payloadType": request.PayloadType,
				"reason":      "card is NDEF read-only",
			},
		}, nil
	}

	ndefMessage := buildNDEFMessage(request)
	tlv := buildType2TLV(ndefMessage)
	if len(tlv) > capability.DataAreaBytes {
		return &CardWriteResult{
			SessionID: session.ID,
			Reader:    session.ReaderName,
			Operation: request.Operation,
			Accepted:  false,
			Details: map[string]string{
				"driver":      d.DriverName(),
				"profile":     request.Profile,
				"mediaType":   request.MediaType,
				"payloadType": request.PayloadType,
				"reason":      fmt.Sprintf("payload requires %d bytes but card data area is %d bytes", len(tlv), capability.DataAreaBytes),
			},
		}, nil
	}

	if err := writeType2Pages(card, type2UserDataPage, tlv); err != nil {
		return nil, err
	}
	if err := card.Disconnect(scard.ResetCard); err != nil {
		return nil, err
	}
	card = nil

	time.Sleep(120 * time.Millisecond)
	if err := d.ensureCardPresentLocked(session.ReaderName); err != nil {
		return nil, err
	}

	verifyCard, err := d.ctx.Connect(session.ReaderName, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return nil, err
	}
	defer verifyCard.Disconnect(scard.LeaveCard)

	if err := verifyType2Write(verifyCard, tlv); err != nil {
		return nil, err
	}

	result := &CardWriteResult{
		SessionID: session.ID,
		Reader:    session.ReaderName,
		Operation: request.Operation,
		Accepted:  true,
		Details: map[string]string{
			"driver":      d.DriverName(),
			"profile":     request.Profile,
			"mediaType":   request.MediaType,
			"payloadType": request.PayloadType,
			"ndefBytes":   fmt.Sprintf("%d", len(ndefMessage)),
			"tlvBytes":    fmt.Sprintf("%d", len(tlv)),
			"pagesWritten": fmt.Sprintf("%d", requiredType2Pages(len(tlv))),
		},
	}

	d.emit(Event{
		Type:      "card.write.complete",
		Status:    "ok",
		Reader:    &Reader{Name: session.ReaderName, Driver: d.DriverName(), CardPreset: true},
		SessionID: session.ID,
		Payload: map[string]any{
			"operation":   request.Operation,
			"profile":     request.Profile,
			"payloadType": request.PayloadType,
		},
	})
	return result, nil
}

func (d *PCSCDarwinDriver) Events() <-chan Event {
	return d.events
}

func (d *PCSCDarwinDriver) Close() error {
	close(d.stop)
	if d.ctx != nil {
		return d.ctx.Release()
	}
	return nil
}

func (d *PCSCDarwinDriver) monitor() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !d.opMu.TryLock() {
				continue
			}

			readers, err := d.listReadersLocked()
			d.opMu.Unlock()
			if err != nil {
				d.emit(Event{Type: "error", Status: "error", Payload: map[string]any{"message": err.Error()}})
				continue
			}

			currentSet := ""
			if len(readers) > 0 {
				currentSet = readers[0].Name
			}

			if currentSet != d.lastReaderSet {
				status := "offline"
				var reader *Reader
				if len(readers) > 0 {
					status = "ready"
					reader = &readers[0]
				}
				d.emit(Event{Type: "reader.status", Status: status, Reader: reader})
				d.lastReaderSet = currentSet
			}

			cardPresent := len(readers) > 0 && readers[0].CardPreset
			if cardPresent != d.lastCardPresent {
				eventType := "card.removed"
				if cardPresent {
					eventType = "card.present"
				}
				d.emit(Event{Type: eventType, Status: map[bool]string{true: "ready", false: "idle"}[cardPresent], Reader: firstReader(readers)})
				d.lastCardPresent = cardPresent
			}
		case <-d.stop:
			close(d.events)
			return
		}
	}
}

func (d *PCSCDarwinDriver) emit(event Event) {
	event.At = time.Now().UTC().Format(time.RFC3339)
	select {
	case d.events <- event:
	default:
	}
}

func (d *PCSCDarwinDriver) ensureCardPresentLocked(readerName string) error {
	states := []scard.ReaderState{{
		Reader:       readerName,
		CurrentState: scard.StateUnaware,
	}}

	err := d.ctx.GetStatusChange(states, 0)
	if err != nil {
		if errors.Is(err, scard.ErrTimeout) {
			return scard.ErrNoSmartcard
		}
		return err
	}

	eventState := states[0].EventState
	if eventState&scard.StatePresent == 0 {
		if eventState&scard.StateUnavailable != 0 {
			return scard.ErrReaderUnavailable
		}
		if eventState&scard.StateMute != 0 {
			return scard.ErrUnresponsiveCard
		}
		return scard.ErrNoSmartcard
	}

	return nil
}

func (d *PCSCDarwinDriver) resolveReaderLocked(readerName string) (string, error) {
	readers, err := d.ctx.ListReaders()
	if err != nil {
		if errors.Is(err, scard.ErrNoReadersAvailable) {
			return "", errors.New("no readers available")
		}
		return "", err
	}
	if len(readers) == 0 {
		return "", errors.New("no readers available")
	}
	if readerName == "" {
		return readers[0], nil
	}
	for _, reader := range readers {
		if reader == readerName {
			return reader, nil
		}
	}
	return "", errors.New("reader not found")
}

func parseUID(response []byte) string {
	if len(response) >= 2 {
		response = response[:len(response)-2]
	}
	return strings.ToUpper(hex.EncodeToString(response))
}

func firstReader(readers []Reader) *Reader {
	if len(readers) == 0 {
		return nil
	}
	reader := readers[0]
	return &reader
}