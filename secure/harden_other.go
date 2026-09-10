//go:build !linux && !darwin && !freebsd && !windows

package secure

func HardenProcess() error { return ErrUnsupportedPlatform }
