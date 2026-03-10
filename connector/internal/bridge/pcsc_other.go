//go:build !darwin && !windows && !linux

package bridge

import "errors"

func NewPCSCDriver() (*MockDriver, error) {
	return nil, errors.New("pcsc driver is not wired for this build target in the current repository")
}