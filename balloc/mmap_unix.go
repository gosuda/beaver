//go:build unix

package balloc

import "syscall"

// mmapAnon reserves and commits length bytes of anonymous, zero-filled
// memory outside the Go heap.
func mmapAnon(length int) ([]byte, error) {
	return syscall.Mmap(
		-1,
		0,
		length,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
}

// munmapAnon releases memory obtained from mmapAnon.
func munmapAnon(mem []byte) error {
	return syscall.Munmap(mem)
}
