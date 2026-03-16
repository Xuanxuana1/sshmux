package proxy

import "errors"

var (
	// ErrNotConnected indicates the SSH master is not connected.
	ErrNotConnected = errors.New("SSH master not connected")
	// ErrPortOccupied indicates the requested port is already in use.
	ErrPortOccupied = errors.New("port already in use")
	// ErrAdapterFailed indicates the HTTP proxy adapter failed to start.
	ErrAdapterFailed = errors.New("HTTP proxy adapter failed")
)
