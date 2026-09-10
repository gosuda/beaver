//go:build freebsd

package secure

import (
	"os"

	"golang.org/x/sys/unix"
)

func platformAlloc(size int, guard bool) (mapping, data []byte, err error) {
	if !guard {
		d, err := unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
		return d, d, err
	}
	ps := os.Getpagesize()
	inner := (size + ps - 1) / ps * ps
	m, err := unix.Mmap(-1, 0, inner+2*ps, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	if err := unix.Mprotect(m[:ps], unix.PROT_NONE); err != nil {
		_ = unix.Munmap(m)
		return nil, nil, err
	}
	if err := unix.Mprotect(m[ps+inner:], unix.PROT_NONE); err != nil {
		_ = unix.Munmap(m)
		return nil, nil, err
	}
	return m, m[ps : ps+size : ps+inner], nil
}

func platformFree(mapping []byte) error {
	return unix.Munmap(mapping)
}

// FreeBSD: MADV_NOCORE is the equivalent of Linux MADV_DONTDUMP.
func lockPages(b []byte) error {
	if err := unix.Mlock(b); err != nil {
		return err
	}
	if err := unix.Madvise(b, unix.MADV_NOCORE); err != nil {
		_ = unix.Munlock(b)
		return err
	}
	return nil
}

func unlockPages(b []byte) {
	_ = unix.Madvise(b, unix.MADV_CORE)
	_ = unix.Munlock(b)
}
