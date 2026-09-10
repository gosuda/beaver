#include "textflag.h"

// func clflushRange(ptr unsafe.Pointer, n uintptr)
//
// CLFLUSH writes each 64-byte cache line back to DRAM and evicts it from
// all cache levels; SFENCE orders the flushes before returning.
TEXT ·clflushRange(SB), NOSPLIT|NOFRAME, $0-16
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), CX
	ADDQ AX, CX
loop:
	CMPQ AX, CX
	JAE  fence
	CLFLUSH (AX)
	ADDQ $64, AX
	JMP  loop
fence:
	SFENCE
	RET
