// +build gc
// +build !noasm

#include "textflag.h"

// Register allocation.
#define dst	R0
#define dstorig	R1
#define src	R2
#define dstend	R3
#define srcend	R4
#define match	R5	// Match address.
#define token	R6
#define len	R7	// Literal and match lengths.
#define offset	R6	// Match offset; overlaps with token.
#define tmp1	R8
#define tmp2	R9
#define tmp3	R12

#define minMatch	$4

// func decodeBlockNodict(dst, src []byte) int
TEXT ·decodeBlockNodict(SB), NOFRAME+NOSPLIT, $-4-28
	MOVW dst_base  +0(FP), dst
	MOVW dst_len   +4(FP), dstend
	MOVW src_base +12(FP), src
	MOVW src_len  +16(FP), srcend

	CMP $0, srcend
	BEQ shortSrc

	ADD dst, dstend
	ADD src, srcend

	MOVW dst, dstorig

loop:
	// Read token. Extract literal length.
	MOVBU.P 1(src), token
	MOVW    token >> 4, len
	CMP     $15, len
	BNE     readLitlenDone

readLitlenLoop:
	CMP     src, srcend
	BEQ     shortSrc
	MOVBU.P 1(src), tmp1
	ADD     tmp1, len
	CMP     $255, tmp1
	BEQ     readLitlenLoop

readLitlenDone:
	CMP $0, len
	BEQ copyLiteralDone

	// Bounds check dst+len and src+len.
	ADD    dst, len, tmp1
	CMP    dstend, tmp1
	//BHI  shortDst	// Uncomment for distinct error codes.
	ADD    src, len, tmp2
	CMP.LS srcend, tmp2
	BHI    shortSrc

	// Copy literal.
	CMP $4, len
	BLO copyLiteralFinish

	// Copy 0-3 bytes until src is aligned.
	TST        $1, src
	MOVBU.NE.P 1(src), tmp1
	MOVB.NE.P  tmp1, 1(dst)
	SUB.NE     $1, len

	TST        $2, src
	MOVHU.NE.P 2(src), tmp2
	MOVB.NE.P  tmp2, 1(dst)
	MOVW.NE    tmp2 >> 8, tmp1
	MOVB.NE.P  tmp1, 1(dst)
	SUB.NE     $2, len

	B copyLiteralLoopCond

copyLiteralLoop:
	// Aligned load, unaligned write.
	MOVW.P 4(src), tmp1
	MOVW   tmp1 >>  8, tmp2
	MOVB   tmp2, 1(dst)
	MOVW   tmp1 >> 16, tmp3
	MOVB   tmp3, 2(dst)
	MOVW   tmp1 >> 24, tmp2
	MOVB   tmp2, 3(dst)
	MOVB.P tmp1, 4(dst)
copyLiteralLoopCond:
	// Loop until len-4 < 0.
	SUB.S  $4, len
	BPL    copyLiteralLoop

	// Restore len, which is now negative.
	ADD $4, len

copyLiteralFinish:
	// Copy remaining 0-3 bytes.
	TST        $2, len
	MOVHU.NE.P 2(src), tmp2
	MOVB.NE.P  tmp2, 1(dst)
	MOVW.NE    tmp2 >> 8, tmp1
	MOVB.NE.P  tmp1, 1(dst)
	TST        $1, len
	MOVBU.NE.P 1(src), tmp1
	MOVB.NE.P  tmp1, 1(dst)

copyLiteralDone:
	CMP src, srcend
	BEQ end

	// Initial part of match length.
	// This frees up the token register for reuse as offset.
	AND $15, token, len

	// Read offset.
	ADD   $2, src
	CMP   srcend, src
	BHI   shortSrc
	MOVBU -2(src), offset
	MOVBU -1(src), tmp1
	ORR   tmp1 << 8, offset
	CMP   $0, offset
	BEQ   corrupt

	// Read rest of match length.
	CMP $15, len
	BNE readMatchlenDone

readMatchlenLoop:
	CMP     src, srcend
	BEQ     shortSrc
	MOVBU.P 1(src), tmp1
	ADD     tmp1, len
	CMP     $255, tmp1
	BEQ     readMatchlenLoop

readMatchlenDone:
	// Bounds check dst+len+minMatch and match = dst-offset.
	ADD    dst, len, tmp1
	ADD    minMatch, tmp1
	CMP    dstend, tmp1
	//BHI  shortDst	// Uncomment for distinct error codes.
	SUB    offset, dst, match
	CMP.LS match, dstorig
	BHI    corrupt

	// Since len+minMatch is at least four, we can do a 4× unrolled
	// byte copy loop. Using MOVW instead of four byte loads is faster,
	// but to remain portable we'd have to align match first, which is
	// too expensive. By alternating loads and stores, we also handle
	// the case offset < 4.
copyMatch4:
	SUB.S   $4, len
	MOVBU.P 4(match), tmp1
	MOVB.P  tmp1, 4(dst)
	MOVBU   -3(match), tmp2
	MOVB    tmp2, -3(dst)
	MOVBU   -2(match), tmp3
	MOVB    tmp3, -2(dst)
	MOVBU   -1(match), tmp1
	MOVB    tmp1, -1(dst)
	BPL     copyMatch4

	// Restore len, which is now negative.
	ADD.S $4, len
	BEQ   copyMatchDone

copyMatch:
	// Finish with a byte-at-a-time copy.
	SUB.S   $1, len
	MOVBU.P 1(match), tmp2
	MOVB.P  tmp2, 1(dst)
	BNE     copyMatch

copyMatchDone:
	CMP src, srcend
	BNE loop

end:
	SUB  dstorig, dst, tmp1
	MOVW tmp1, ret+24(FP)
	RET

	// The three error cases have distinct labels so we can put different
	// return codes here when debugging, or if the error returns need to
	// be changed.
shortDst:
shortSrc:
corrupt:
	MOVW $-1, tmp1
	MOVW tmp1, ret+24(FP)
	RET
