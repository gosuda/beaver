//go:build linux

package secure

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// TestPagesLockedAndDumpExcluded verifies against the kernel itself that
// the OS-level promises hold: the buffer's VMA must show locked pages
// (Locked/VmLck, or the lo VmFlag) and the do-not-dump flag (dd).
func TestPagesLockedAndDumpExcluded(t *testing.T) {
	b, err := NewBuffer(4096, WithLock())
	if err != nil {
		t.Skipf("page locking unavailable (RLIMIT_MEMLOCK?): %v", err)
	}
	defer b.Destroy()
	// Fault the anonymous pages in; the locked counter only covers
	// populated pages.
	for i := range b.Bytes() {
		b.Bytes()[i] = 0x11
	}
	addr := uintptr(unsafe.Pointer(&b.Bytes()[0]))
	r := findSmapRegion(addr)
	if !r.ok {
		t.Skip("could not locate buffer VMA in /proc/self/smaps")
	}
	fields := " " + r.flags + " "
	if r.vmLckKB*1024 < 4096 && !strings.Contains(fields, " lo ") {
		t.Fatalf("Locked = %d kB, VmFlags %q: pages not locked against swap", r.vmLckKB, r.flags)
	}
	if !strings.Contains(fields, " dd ") {
		t.Fatalf("VmFlags %q missing dd (MADV_DONTDUMP)", r.flags)
	}
}

type smapRegion struct {
	vmLckKB uint64
	flags   string
	ok      bool
}

func findSmapRegion(addr uintptr) smapRegion {
	data, err := os.ReadFile("/proc/self/smaps")
	if err != nil {
		return smapRegion{}
	}
	var r smapRegion
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		if start, end, ok := parseSmapHeader(line); ok {
			in = start <= addr && addr < end
			if in {
				r.ok = true
			}
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, "VmLck:") || strings.HasPrefix(line, "Locked:") {
			if f := strings.Fields(line); len(f) >= 2 {
				r.vmLckKB, _ = strconv.ParseUint(f[1], 10, 64)
			}
		}
		if strings.HasPrefix(line, "VmFlags:") {
			r.flags = strings.TrimSpace(strings.TrimPrefix(line, "VmFlags:"))
		}
	}
	return r
}

func parseSmapHeader(line string) (start, end uintptr, ok bool) {
	dash := strings.IndexByte(line, '-')
	if dash < 1 {
		return 0, 0, false
	}
	rest := line[dash+1:]
	space := strings.IndexByte(rest, ' ')
	if space < 1 {
		return 0, 0, false
	}
	s, err1 := strconv.ParseUint(line[:dash], 16, 64)
	e, err2 := strconv.ParseUint(rest[:space], 16, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return uintptr(s), uintptr(e), true
}
