#include "textflag.h"

// func memzero(ptr unsafe.Pointer, n uintptr)
//
// Assembly so the compiler cannot eliminate the stores as dead writes.
TEXT ·memzero(SB), NOSPLIT|NOFRAME, $0-16
	MOVD ptr+0(FP), R0
	MOVD n+8(FP), R1
words:
	CMP  $8, R1
	BLT  bytes
	MOVD ZR, (R0)
	ADD  $8, R0
	SUB  $8, R1
	B    words
bytes:
	CBZ  R1, done
	MOVB ZR, (R0)
	ADD  $1, R0
	SUB  $1, R1
	B    bytes
done:
	RET
