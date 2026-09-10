#include "textflag.h"

// func memzero(ptr unsafe.Pointer, n uintptr)
//
// Assembly so the compiler cannot eliminate the stores as dead writes.
TEXT ·memzero(SB), NOSPLIT|NOFRAME, $0-16
	MOVQ ptr+0(FP), DI
	MOVQ n+8(FP), CX
	XORQ AX, AX
words:
	CMPQ CX, $8
	JB   bytes
	MOVQ AX, (DI)
	ADDQ $8, DI
	SUBQ $8, CX
	JMP  words
bytes:
	TESTQ CX, CX
	JZ    done
	MOVB  AL, (DI)
	INCQ  DI
	DECQ  CX
	JMP   bytes
done:
	RET
