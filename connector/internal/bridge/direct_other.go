//go:build !windows

package bridge

import (
	"context"
	"errors"
)

func NewDirectDriver() (*DirectDriver, error) {
	return nil, errors.New("direct driver is only available on windows")
}

type DirectDriver struct{}

func (d *DirectDriver) DriverName() string                                                        { return "direct" }
func (d *DirectDriver) Health(context.Context) map[string]any                                     { return nil }
func (d *DirectDriver) ListReaders(context.Context) ([]Reader, error)                             { return nil, errors.New("unsupported") }
func (d *DirectDriver) ConnectSession(context.Context, string) (*Session, error)                  { return nil, errors.New("unsupported") }
func (d *DirectDriver) ReadCard(context.Context, *Session, string) (*CardReadResult, error)       { return nil, errors.New("unsupported") }
func (d *DirectDriver) WriteCard(context.Context, *Session, *WriteRequest) (*CardWriteResult, error) { return nil, errors.New("unsupported") }
func (d *DirectDriver) Events() <-chan Event                                                      { return nil }
func (d *DirectDriver) Close() error                                                              { return nil }
