#include "textflag.h"

// func dcCivacRange(ptr unsafe.Pointer, n uintptr)
//
// DC CIVAC cleans and invalidates each data cache line to the point of
// coherency; DSB waits for completion. Assumes 64-byte lines (some cores,
// e.g. Apple silicon, use 128 — those are flushed partially).
TEXT ·dcCivacRange(SB), NOSPLIT|NOFRAME, $0-16
	MOVD ptr+0(FP), R0
	MOVD n+8(FP), R1
	ADD  R0, R1
loop:
	CMP  R0, R1
	BLS  barrier
	DC   CIVAC, R0
	ADD  $64, R0
	B    loop
barrier:
	DSB  $15
	RET
