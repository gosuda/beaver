#include "textflag.h"

// func ntFill64(ptr unsafe.Pointer, n uintptr, val uint64)
//
// Fills n bytes (multiple of 8) with val using MOVNTI non-temporal stores,
// which bypass the cache via write-combining buffers. Test-only attack
// primitive: plants data where a cache-level wipe might miss it.
TEXT ·ntFill64(SB), NOSPLIT|NOFRAME, $0-24
	MOVQ ptr+0(FP), DI
	MOVQ n+8(FP), CX
	MOVQ val+16(FP), AX
loop:
	TESTQ CX, CX
	JZ    done
	MOVNTIQ AX, (DI)
	ADDQ $8, DI
	SUBQ $8, CX
	JMP   loop
done:
	SFENCE
	RET
