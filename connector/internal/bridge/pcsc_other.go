//go:build !darwin && !windows && !linux

package bridge

import (
	"context"
	"errors"
)

func NewPCSCDriver() (*PCSCDriver, error) {
	return nil, errors.New("pcsc driver is not wired for this build target in the current repository")
}

type PCSCDriver struct{}

func (d *PCSCDriver) DriverName() string                                            { return "pcsc" }
func (d *PCSCDriver) Health(context.Context) map[string]any                          { return nil }
func (d *PCSCDriver) ListReaders(context.Context) ([]Reader, error)                  { return nil, errors.New("unsupported") }
func (d *PCSCDriver) ConnectSession(context.Context, string) (*Session, error)       { return nil, errors.New("unsupported") }
func (d *PCSCDriver) ReadCard(context.Context, *Session, string) (*CardReadResult, error) { return nil, errors.New("unsupported") }
func (d *PCSCDriver) WriteCard(context.Context, *Session, *WriteRequest) (*CardWriteResult, error) { return nil, errors.New("unsupported") }
func (d *PCSCDriver) Events() <-chan Event                                           { return nil }
func (d *PCSCDriver) Close() error                                                   { return nil }