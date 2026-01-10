//go:build linux

package supervisor

import "syscall"

var shutdownSignal = syscall.SIGTERM
