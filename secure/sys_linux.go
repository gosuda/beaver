//go:build linux

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

func lockPages(b []byte) error {
	if err := unix.Mlock(b); err != nil {
		return err
	}
	if err := unix.Madvise(b, unix.MADV_DONTDUMP); err != nil {
		_ = unix.Munlock(b)
		return err
	}
	// Keep the pages out of KSM, a cross-process side channel.
	if err := unix.Madvise(b, unix.MADV_UNMERGEABLE); err != nil {
		_ = unix.Madvise(b, unix.MADV_DODUMP)
		_ = unix.Munlock(b)
		return err
	}
	return nil
}

func unlockPages(b []byte) {
	// UNMERGEABLE is left in place: the pages are about to be unmapped,
	// and marking them mergeable would opt back into KSM.
	_ = unix.Madvise(b, unix.MADV_DODUMP)
	_ = unix.Munlock(b)
}
