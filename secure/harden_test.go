package secure

import "testing"

func TestHardenProcess(t *testing.T) {
	if err := HardenProcess(); err != nil {
		t.Fatalf("HardenProcess: %v", err)
	}
}

func TestGuardPagesBasic(t *testing.T) {
	b, err := NewBuffer(1000, WithGuardPages(), WithZeroize(), WithBestEffortLock())
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	if b.Len() != 1000 {
		t.Fatalf("Len = %d, want 1000", b.Len())
	}
	if len(b.mapping) <= len(b.Bytes()) {
		t.Fatal("guard pages: mapping should be larger than the user region")
	}
	data := b.Bytes()
	for i := range data {
		data[i] = byte(i)
	}
	if err := b.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}
