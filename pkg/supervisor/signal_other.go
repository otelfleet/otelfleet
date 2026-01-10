//go:build !linux

package supervisor

import "os"

var shutdownSignal = os.Interrupt
