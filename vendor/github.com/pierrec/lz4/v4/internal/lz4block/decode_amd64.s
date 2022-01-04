// +build !appengine
// +build gc
// +build !noasm

#include "textflag.h"

// AX scratch
// BX scratch
// CX scratch
// DX token
//
// DI &dst
// SI &src
// R8 &dst + len(dst)
// R9 &src + len(src)
// R11 &dst
// R12 short output end
// R13 short input end
// R14 &dict
// R15 &dict + len(dict)

// func decodeBlock(dst, src, dict []byte) int
TEXT ·decodeBlock(SB), NOSPLIT, $48-80
	MOVQ dst_base+0(FP), DI
	MOVQ DI, R11
	MOVQ dst_len+8(FP), R8
	ADDQ DI, R8

	MOVQ src_base+24(FP), SI
	MOVQ src_len+32(FP), R9
	CMPQ R9, $0
	JE   err_corrupt
	ADDQ SI, R9

	MOVQ dict_base+48(FP), R14
	MOVQ dict_len+56(FP), R15
	ADDQ R14, R15

	// shortcut ends
	// short output end
	MOVQ R8, R12
	SUBQ $32, R12
	// short input end
	MOVQ R9, R13
	SUBQ $16, R13

loop:
	// for si < len(src)
	CMPQ SI, R9
	JGE end

	// token := uint32(src[si])
	MOVBQZX (SI), DX
	INCQ SI

	// lit_len = token >> 4
	// if lit_len > 0
	// CX = lit_len
	MOVQ DX, CX
	SHRQ $4, CX

	// if lit_len != 0xF
	CMPQ CX, $0xF
	JEQ lit_len_loop_pre
	CMPQ DI, R12
	JGE lit_len_loop_pre
	CMPQ SI, R13
	JGE lit_len_loop_pre

	// copy shortcut

	// A two-stage shortcut for the most common case:
	// 1) If the literal length is 0..14, and there is enough space,
	// enter the shortcut and copy 16 bytes on behalf of the literals
	// (in the fast mode, only 8 bytes can be safely copied this way).
	// 2) Further if the match length is 4..18, copy 18 bytes in a similar
	// manner; but we ensure that there's enough space in the output for
	// those 18 bytes earlier, upon entering the shortcut (in other words,
	// there is a combined check for both stages).

	// copy literal
	MOVOU (SI), X0
	MOVOU X0, (DI)
	ADDQ CX, DI
	ADDQ CX, SI

	MOVQ DX, CX
	ANDQ $0xF, CX

	// The second stage: prepare for match copying, decode full info.
	// If it doesn't work out, the info won't be wasted.
	// offset := uint16(data[:2])
	MOVWQZX (SI), DX
	ADDQ $2, SI

	MOVQ DI, AX
	SUBQ DX, AX
	CMPQ AX, DI
	JGT err_short_buf

	// if we can't do the second stage then jump straight to read the
	// match length, we already have the offset.
	CMPQ CX, $0xF
	JEQ match_len_loop_pre
	CMPQ DX, R11
	JLT match_len_loop_pre
	CMPQ DX, $8
	JLT match_len_loop_pre
	CMPQ AX, R11
	JLT err_short_buf

	// memcpy(op + 0, match + 0, 8);
	MOVQ (AX), BX
	MOVQ BX, (DI)
	// memcpy(op + 8, match + 8, 8);
	MOVQ 8(AX), BX
	MOVQ BX, 8(DI)
	// memcpy(op +16, match +16, 2);
	MOVW 16(AX), BX
	MOVW BX, 16(DI)

	LEAQ 4(DI)(CX*1), DI // minmatch

	// shortcut complete, load next token
	JMP loop

lit_len_loop_pre:
	// if lit_len > 0
	CMPQ CX, $0
	JEQ offset
	CMPQ CX, $0xF
	JNE copy_literal

lit_len_loop:
	// for src[si] == 0xFF
	CMPB (SI), $0xFF
	JNE lit_len_finalise

	// bounds check src[si+1]
	LEAQ 1(SI), AX
	CMPQ AX, R9
	JGT err_short_buf

	// lit_len += 0xFF
	ADDQ $0xFF, CX
	INCQ SI
	JMP lit_len_loop

lit_len_finalise:
	// lit_len += int(src[si])
	// si++
	MOVBQZX (SI), AX
	ADDQ AX, CX
	INCQ SI

copy_literal:
	// bounds check src and dst
	LEAQ (SI)(CX*1), AX
	CMPQ AX, R9
	JGT err_short_buf

	LEAQ (DI)(CX*1), AX
	CMPQ AX, R8
	JGT err_short_buf

	// whats a good cut off to call memmove?
	CMPQ CX, $16
	JGT memmove_lit

	// if len(dst[di:]) < 16
	MOVQ R8, AX
	SUBQ DI, AX
	CMPQ AX, $16
	JLT memmove_lit

	// if len(src[si:]) < 16
	MOVQ R9, AX
	SUBQ SI, AX
	CMPQ AX, $16
	JLT memmove_lit

	MOVOU (SI), X0
	MOVOU X0, (DI)

	ADDQ CX, SI
	ADDQ CX, DI

	JMP finish_lit_copy

memmove_lit:
	// memmove(to, from, len)
	MOVQ DI, 0(SP)
	MOVQ SI, 8(SP)
	MOVQ CX, 16(SP)

	// Spill registers. Increment SI, DI now so we don't need to save CX.
	ADDQ CX, DI
	ADDQ CX, SI
	MOVQ DI, 24(SP)
	MOVQ SI, 32(SP)
	MOVL DX, 40(SP)

	CALL runtime·memmove(SB)

	// restore registers
	MOVQ 24(SP), DI
	MOVQ 32(SP), SI
	MOVL 40(SP), DX

	// recalc initial values
	MOVQ dst_base+0(FP), R8
	MOVQ R8, R11
	ADDQ dst_len+8(FP), R8
	MOVQ src_base+24(FP), R9
	ADDQ src_len+32(FP), R9
	MOVQ R8, R12
	SUBQ $32, R12
	MOVQ R9, R13
	SUBQ $16, R13

finish_lit_copy:
	CMPQ SI, R9
	JGE end

offset:
	// CX := mLen
	// free up DX to use for offset
	MOVQ DX, CX

	LEAQ 2(SI), AX
	CMPQ AX, R9
	JGT err_short_buf

	// offset
	// DX := int(src[si]) | int(src[si+1])<<8
	MOVWQZX (SI), DX
	ADDQ $2, SI

	// 0 offset is invalid
	CMPQ DX, $0
	JEQ err_corrupt

	ANDB $0xF, CX

match_len_loop_pre:
	// if mlen != 0xF
	CMPB CX, $0xF
	JNE copy_match

match_len_loop:
	// for src[si] == 0xFF
	// lit_len += 0xFF
	CMPB (SI), $0xFF
	JNE match_len_finalise

	// bounds check src[si+1]
	LEAQ 1(SI), AX
	CMPQ AX, R9
	JGT err_short_buf

	ADDQ $0xFF, CX
	INCQ SI
	JMP match_len_loop

match_len_finalise:
	// lit_len += int(src[si])
	// si++
	MOVBQZX (SI), AX
	ADDQ AX, CX
	INCQ SI

copy_match:
	// mLen += minMatch
	ADDQ $4, CX

	// check we have match_len bytes left in dst
	// di+match_len < len(dst)
	LEAQ (DI)(CX*1), AX
	CMPQ AX, R8
	JGT err_short_buf

	// DX = offset
	// CX = match_len
	// BX = &dst + (di - offset)
	MOVQ DI, BX
	SUBQ DX, BX

	// check BX is within dst
	// if BX < &dst
	CMPQ BX, R11
	JLT copy_match_from_dict

	// if offset + match_len < di
	LEAQ (BX)(CX*1), AX
	CMPQ DI, AX
	JGT copy_interior_match

	// AX := len(dst[:di])
	// MOVQ DI, AX
	// SUBQ R11, AX

	// copy 16 bytes at a time
	// if di-offset < 16 copy 16-(di-offset) bytes to di
	// then do the remaining

copy_match_loop:
	// for match_len >= 0
	// dst[di] = dst[i]
	// di++
	// i++
	MOVB (BX), AX
	MOVB AX, (DI)
	INCQ DI
	INCQ BX
	DECQ CX

	CMPQ CX, $0
	JGT copy_match_loop

	JMP loop

copy_interior_match:
	CMPQ CX, $16
	JGT memmove_match

	// if len(dst[di:]) < 16
	MOVQ R8, AX
	SUBQ DI, AX
	CMPQ AX, $16
	JLT memmove_match

	MOVOU (BX), X0
	MOVOU X0, (DI)

	ADDQ CX, DI
	JMP loop

copy_match_from_dict:
	// CX = match_len
	// BX = &dst + (di - offset)

	// AX = offset - di = dict_bytes_available => count of bytes potentially covered by the dictionary
	MOVQ R11, AX
	SUBQ BX, AX

	// BX = &dict_end - dict_bytes_available
	MOVQ R15, BX
	SUBQ AX, BX

	// check BX is within dict
	// if BX < &dict
	CMPQ BX, R14
	JLT err_short_dict

	// if match_len > dict_bytes_available, match fits entirely within external dictionary : just copy
	CMPQ CX, AX
	JLT memmove_match

	// The match stretches over the dictionary and our block
	// 1) copy what comes from the dictionary
	// AX = dict_bytes_available = copy_size
	// BX = &dict_end - copy_size
	// CX = match_len

	// memmove(to, from, len)
	MOVQ DI, 0(SP)
	MOVQ BX, 8(SP)
	MOVQ AX, 16(SP)
	// store extra stuff we want to recover
	// spill
	MOVQ DI, 24(SP)
	MOVQ SI, 32(SP)
	MOVQ CX, 40(SP)
	CALL runtime·memmove(SB)

	// restore registers
	MOVQ 16(SP), AX // copy_size
	MOVQ 24(SP), DI
	MOVQ 32(SP), SI
	MOVQ 40(SP), CX // match_len

	// recalc initial values
	MOVQ dst_base+0(FP), R8
	MOVQ R8, R11 // TODO: make these sensible numbers
	ADDQ dst_len+8(FP), R8
	MOVQ src_base+24(FP), R9
	ADDQ src_len+32(FP), R9
	MOVQ dict_base+48(FP), R14
	MOVQ dict_len+56(FP), R15
	ADDQ R14, R15
	MOVQ R8, R12
	SUBQ $32, R12
	MOVQ R9, R13
	SUBQ $16, R13

	// di+=copy_size
	ADDQ AX, DI

	// 2) copy the rest from the current block
	// CX = match_len - copy_size = rest_size
	SUBQ AX, CX
	MOVQ R11, BX

	// check if we have a copy overlap
	// AX = &dst + rest_size
	MOVQ CX, AX
	ADDQ BX, AX
	// if &dst + rest_size > di, copy byte by byte
	CMPQ AX, DI

	JGT copy_match_loop

memmove_match:
	// memmove(to, from, len)
	MOVQ DI, 0(SP)
	MOVQ BX, 8(SP)
	MOVQ CX, 16(SP)

	// Spill registers. Increment DI now so we don't need to save CX.
	ADDQ CX, DI
	MOVQ DI, 24(SP)
	MOVQ SI, 32(SP)

	CALL runtime·memmove(SB)

	// restore registers
	MOVQ 24(SP), DI
	MOVQ 32(SP), SI

	// recalc initial values
	MOVQ dst_base+0(FP), R8
	MOVQ R8, R11 // TODO: make these sensible numbers
	ADDQ dst_len+8(FP), R8
	MOVQ src_base+24(FP), R9
	ADDQ src_len+32(FP), R9
	MOVQ R8, R12
	SUBQ $32, R12
	MOVQ R9, R13
	SUBQ $16, R13
	MOVQ dict_base+48(FP), R14
	MOVQ dict_len+56(FP), R15
	ADDQ R14, R15

	JMP loop

err_corrupt:
	MOVQ $-1, ret+72(FP)
	RET

err_short_buf:
	MOVQ $-2, ret+72(FP)
	RET

err_short_dict:
	MOVQ $-3, ret+72(FP)
	RET

end:
	SUBQ R11, DI
	MOVQ DI, ret+72(FP)
	RET
