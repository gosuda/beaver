//go:build !linux && !darwin && !freebsd && !windows

package secure

func platformAlloc(size int, guard bool) (mapping, data []byte, err error) {
	return nil, nil, ErrUnsupportedPlatform
}

func platformFree(mapping []byte) error {
	return ErrUnsupportedPlatform
}

func lockPages(b []byte) error {
	return ErrUnsupportedPlatform
}

func unlockPages(b []byte) {}
