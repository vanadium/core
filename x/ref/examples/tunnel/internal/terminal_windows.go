// +build windows

package internal

import (
	"errors"

	"v.io/v23/context"
)

func SetWindowSize(fd uintptr, ws Winsize) error {
	return errors.New("not implemented")
}

func GetWindowSize() (*Winsize, error) {
	return nil, errors.New("not implemented")
}

func EnterRawTerminalMode(ctx *context.T) string {
	panic("not implemented")
	return ""
}

func RestoreTerminalSettings(ctx *context.T, saved string) {
	panic("not implemented")
}
