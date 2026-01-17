//go:build !insecure

package main

// isSecureMode returns true when built without the insecure tag.
func isSecureMode() bool {
	return true
}
