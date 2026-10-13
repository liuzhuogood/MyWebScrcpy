//go:build !darwin

package desktop

// InitEnvironment is a no-op on non-macOS systems.
func InitEnvironment() {}
