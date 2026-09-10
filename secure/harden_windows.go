//go:build windows

package secure

// HardenProcess is a no-op on Windows: there is no process-wide dumpable
// flag. Per-region dump exclusion is already handled by WithLock through
// WerRegisterExcludedMemoryBlock.
func HardenProcess() error { return nil }
