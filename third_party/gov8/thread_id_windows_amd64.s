//go:build windows && amd64

#include "textflag.h"

// On Windows amd64, GS points at the Thread Environment Block. ClientId is at
// offset 0x40 and its UniqueThread field is the pointer-sized word at 0x48.
// currentThreadID verifies this value against GetCurrentThreadId before using
// the fast path, so a future incompatible layout safely falls back to Win32.
TEXT ·currentThreadIDFast(SB), NOSPLIT, $0-4
	MOVQ	0x48(GS), AX
	MOVL	AX, ret+0(FP)
	RET
