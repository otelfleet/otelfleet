//go:build insecure

package main

// isSecureMode returns false when built with the insecure tag.
func isSecureMode() bool {
	return false
}
