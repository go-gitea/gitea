// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.7,amd64,gc,!purego

#include "textflag.h"

DATA ·AVX2_iv0<>+0x00(SB)/8, $0x6a09e667f3bcc908
DATA ·AVX2_iv0<>+0x08(SB)/8, $0xbb67ae8584caa73b
DATA ·AVX2_iv0<>+0x10(SB)/8, $0x3c6ef372fe94f82b
DATA ·AVX2_iv0<>+0x18(SB)/8, $0xa54ff53a5f1d36f1
GLOBL ·AVX2_iv0<>(SB), (NOPTR+RODATA), $32

DATA ·AVX2_iv1<>+0x00(SB)/8, $0x510e527fade682d1
DATA ·AVX2_iv1<>+0x08(SB)/8, $0x9b05688c2b3e6c1f
DATA ·AVX2_iv1<>+0x10(SB)/8, $0x1f83d9abfb41bd6b
DATA ·AVX2_iv1<>+0x18(SB)/8, $0x5be0cd19137e2179
GLOBL ·AVX2_iv1<>(SB), (NOPTR+RODATA), $32

DATA ·AVX2_c40<>+0x00(SB)/8, $0x0201000706050403
DATA ·AVX2_c40<>+0x08(SB)/8, $0x0a09080f0e0d0c0b
DATA ·AVX2_c40<>+0x10(SB)/8, $0x0201000706050403
DATA ·AVX2_c40<>+0x18(SB)/8, $0x0a09080f0e0d0c0b
GLOBL ·AVX2_c40<>(SB), (NOPTR+RODATA), $32

DATA ·AVX2_c48<>+0x00(SB)/8, $0x0100070605040302
DATA ·AVX2_c48<>+0x08(SB)/8, $0x09080f0e0d0c0b0a
DATA ·AVX2_c48<>+0x10(SB)/8, $0x0100070605040302
DATA ·AVX2_c48<>+0x18(SB)/8, $0x09080f0e0d0c0b0a
GLOBL ·AVX2_c48<>(SB), (NOPTR+RODATA), $32

DATA ·AVX_iv0<>+0x00(SB)/8, $0x6a09e667f3bcc908
DATA ·AVX_iv0<>+0x08(SB)/8, $0xbb67ae8584caa73b
GLOBL ·AVX_iv0<>(SB), (NOPTR+RODATA), $16

DATA ·AVX_iv1<>+0x00(SB)/8, $0x3c6ef372fe94f82b
DATA ·AVX_iv1<>+0x08(SB)/8, $0xa54ff53a5f1d36f1
GLOBL ·AVX_iv1<>(SB), (NOPTR+RODATA), $16

DATA ·AVX_iv2<>+0x00(SB)/8, $0x510e527fade682d1
DATA ·AVX_iv2<>+0x08(SB)/8, $0x9b05688c2b3e6c1f
GLOBL ·AVX_iv2<>(SB), (NOPTR+RODATA), $16

DATA ·AVX_iv3<>+0x00(SB)/8, $0x1f83d9abfb41bd6b
DATA ·AVX_iv3<>+0x08(SB)/8, $0x5be0cd19137e2179
GLOBL ·AVX_iv3<>(SB), (NOPTR+RODATA), $16

DATA ·AVX_c40<>+0x00(SB)/8, $0x0201000706050403
DATA ·AVX_c40<>+0x08(SB)/8, $0x0a09080f0e0d0c0b
GLOBL ·AVX_c40<>(SB), (NOPTR+RODATA), $16

DATA ·AVX_c48<>+0x00(SB)/8, $0x0100070605040302
DATA ·AVX_c48<>+0x08(SB)/8, $0x09080f0e0d0c0b0a
GLOBL ·AVX_c48<>(SB), (NOPTR+RODATA), $16

#define VPERMQ_0x39_Y1_Y1 BYTE $0xc4; BYTE $0xe3; BYTE $0xfd; BYTE $0x00; BYTE $0xc9; BYTE $0x39
#define VPERMQ_0x93_Y1_Y1 BYTE $0xc4; BYTE $0xe3; BYTE $0xfd; BYTE $0x00; BYTE $0xc9; BYTE $0x93
#define VPERMQ_0x4E_Y2_Y2 BYTE $0xc4; BYTE $0xe3; BYTE $0xfd; BYTE $0x00; BYTE $0xd2; BYTE $0x4e
#define VPERMQ_0x93_Y3_Y3 BYTE $0xc4; BYTE $0xe3; BYTE $0xfd; BYTE $0x00; BYTE $0xdb; BYTE $0x93
#define VPERMQ_0x39_Y3_Y3 BYTE $0xc4; BYTE $0xe3; BYTE $0xfd; BYTE $0x00; BYTE $0xdb; BYTE $0x39

#define ROUND_AVX2(m0, m1, m2, m3, t, c40, c48) \
	VPADDQ  m0, Y0, Y0;   \
	VPADDQ  Y1, Y0, Y0;   \
	VPXOR   Y0, Y3, Y3;   \
	VPSHUFD $-79, Y3, Y3; \
	VPADDQ  Y3, Y2, Y2;   \
	VPXOR   Y2, Y1, Y1;   \
	VPSHUFB c40, Y1, Y1;  \
	VPADDQ  m1, Y0, Y0;   \
	VPADDQ  Y1, Y0, Y0;   \
	VPXOR   Y0, Y3, Y3;   \
	VPSHUFB c48, Y3, Y3;  \
	VPADDQ  Y3, Y2, Y2;   \
	VPXOR   Y2, Y1, Y1;   \
	VPADDQ  Y1, Y1, t;    \
	VPSRLQ  $63, Y1, Y1;  \
	VPXOR   t, Y1, Y1;    \
	VPERMQ_0x39_Y1_Y1;    \
	VPERMQ_0x4E_Y2_Y2;    \
	VPERMQ_0x93_Y3_Y3;    \
	VPADDQ  m2, Y0, Y0;   \
	VPADDQ  Y1, Y0, Y0;   \
	VPXOR   Y0, Y3, Y3;   \
	VPSHUFD $-79, Y3, Y3; \
	VPADDQ  Y3, Y2, Y2;   \
	VPXOR   Y2, Y1, Y1;   \
	VPSHUFB c40, Y1, Y1;  \
	VPADDQ  m3, Y0, Y0;   \
	VPADDQ  Y1, Y0, Y0;   \
	VPXOR   Y0, Y3, Y3;   \
	VPSHUFB c48, Y3, Y3;  \
	VPADDQ  Y3, Y2, Y2;   \
	VPXOR   Y2, Y1, Y1;   \
	VPADDQ  Y1, Y1, t;    \
	VPSRLQ  $63, Y1, Y1;  \
	VPXOR   t, Y1, Y1;    \
	VPERMQ_0x39_Y3_Y3;    \
	VPERMQ_0x4E_Y2_Y2;    \
	VPERMQ_0x93_Y1_Y1

#define VMOVQ_SI_X11_0 BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x1E
#define VMOVQ_SI_X12_0 BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x26
#define VMOVQ_SI_X13_0 BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x2E
#define VMOVQ_SI_X14_0 BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x36
#define VMOVQ_SI_X15_0 BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x3E

#define VMOVQ_SI_X11(n) BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x5E; BYTE $n
#define VMOVQ_SI_X12(n) BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x66; BYTE $n
#define VMOVQ_SI_X13(n) BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x6E; BYTE $n
#define VMOVQ_SI_X14(n) BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x76; BYTE $n
#define VMOVQ_SI_X15(n) BYTE $0xC5; BYTE $0x7A; BYTE $0x7E; BYTE $0x7E; BYTE $n

#define VPINSRQ_1_SI_X11_0 BYTE $0xC4; BYTE $0x63; BYTE $0xA1; BYTE $0x22; BYTE $0x1E; BYTE $0x01
#define VPINSRQ_1_SI_X12_0 BYTE $0xC4; BYTE $0x63; BYTE $0x99; BYTE $0x22; BYTE $0x26; BYTE $0x01
#define VPINSRQ_1_SI_X13_0 BYTE $0xC4; BYTE $0x63; BYTE $0x91; BYTE $0x22; BYTE $0x2E; BYTE $0x01
#define VPINSRQ_1_SI_X14_0 BYTE $0xC4; BYTE $0x63; BYTE $0x89; BYTE $0x22; BYTE $0x36; BYTE $0x01
#define VPINSRQ_1_SI_X15_0 BYTE $0xC4; BYTE $0x63; BYTE $0x81; BYTE $0x22; BYTE $0x3E; BYTE $0x01

#define VPINSRQ_1_SI_X11(n) BYTE $0xC4; BYTE $0x63; BYTE $0xA1; BYTE $0x22; BYTE $0x5E; BYTE $n; BYTE $0x01
#define VPINSRQ_1_SI_X12(n) BYTE $0xC4; BYTE $0x63; BYTE $0x99; BYTE $0x22; BYTE $0x66; BYTE $n; BYTE $0x01
#define VPINSRQ_1_SI_X13(n) BYTE $0xC4; BYTE $0x63; BYTE $0x91; BYTE $0x22; BYTE $0x6E; BYTE $n; BYTE $0x01
#define VPINSRQ_1_SI_X14(n) BYTE $0xC4; BYTE $0x63; BYTE $0x89; BYTE $0x22; BYTE $0x76; BYTE $n; BYTE $0x01
#define VPINSRQ_1_SI_X15(n) BYTE $0xC4; BYTE $0x63; BYTE $0x81; BYTE $0x22; BYTE $0x7E; BYTE $n; BYTE $0x01

#define VMOVQ_R8_X15 BYTE $0xC4; BYTE $0x41; BYTE $0xF9; BYTE $0x6E; BYTE $0xF8
#define VPINSRQ_1_R9_X15 BYTE $0xC4; BYTE $0x43; BYTE $0x81; BYTE $0x22; BYTE $0xF9; BYTE $0x01

// load msg: Y12 = (i0, i1, i2, i3)
// i0, i1, i2, i3 must not be 0
#define LOAD_MSG_AVX2_Y12(i0, i1, i2, i3) \
	VMOVQ_SI_X12(i0*8);           \
	VMOVQ_SI_X11(i2*8);           \
	VPINSRQ_1_SI_X12(i1*8);       \
	VPINSRQ_1_SI_X11(i3*8);       \
	VINSERTI128 $1, X11, Y12, Y12

// load msg: Y13 = (i0, i1, i2, i3)
// i0, i1, i2, i3 must not be 0
#define LOAD_MSG_AVX2_Y13(i0, i1, i2, i3) \
	VMOVQ_SI_X13(i0*8);           \
	VMOVQ_SI_X11(i2*8);           \
	VPINSRQ_1_SI_X13(i1*8);       \
	VPINSRQ_1_SI_X11(i3*8);       \
	VINSERTI128 $1, X11, Y13, Y13

// load msg: Y14 = (i0, i1, i2, i3)
// i0, i1, i2, i3 must not be 0
#define LOAD_MSG_AVX2_Y14(i0, i1, i2, i3) \
	VMOVQ_SI_X14(i0*8);           \
	VMOVQ_SI_X11(i2*8);           \
	VPINSRQ_1_SI_X14(i1*8);       \
	VPINSRQ_1_SI_X11(i3*8);       \
	VINSERTI128 $1, X11, Y14, Y14

// load msg: Y15 = (i0, i1, i2, i3)
// i0, i1, i2, i3 must not be 0
#define LOAD_MSG_AVX2_Y15(i0, i1, i2, i3) \
	VMOVQ_SI_X15(i0*8);           \
	VMOVQ_SI_X11(i2*8);           \
	VPINSRQ_1_SI_X15(i1*8);       \
	VPINSRQ_1_SI_X11(i3*8);       \
	VINSERTI128 $1, X11, Y15, Y15

#define LOAD_MSG_AVX2_0_2_4_6_1_3_5_7_8_10_12_14_9_11_13_15() \
	VMOVQ_SI_X12_0;                   \
	VMOVQ_SI_X11(4*8);                \
	VPINSRQ_1_SI_X12(2*8);            \
	VPINSRQ_1_SI_X11(6*8);            \
	VINSERTI128 $1, X11, Y12, Y12;    \
	LOAD_MSG_AVX2_Y13(1, 3, 5, 7);    \
	LOAD_MSG_AVX2_Y14(8, 10, 12, 14); \
	LOAD_MSG_AVX2_Y15(9, 11, 13, 15)

#define LOAD_MSG_AVX2_14_4_9_13_10_8_15_6_1_0_11_5_12_2_7_3() \
	LOAD_MSG_AVX2_Y12(14, 4, 9, 13); \
	LOAD_MSG_AVX2_Y13(10, 8, 15, 6); \
	VMOVQ_SI_X11(11*8);              \
	VPSHUFD     $0x4E, 0*8(SI), X14; \
	VPINSRQ_1_SI_X11(5*8);           \
	VINSERTI128 $1, X11, Y14, Y14;   \
	LOAD_MSG_AVX2_Y15(12, 2, 7, 3)

#define LOAD_MSG_AVX2_11_12_5_15_8_0_2_13_10_3_7_9_14_6_1_4() \
	VMOVQ_SI_X11(5*8);              \
	VMOVDQU     11*8(SI), X12;      \
	VPINSRQ_1_SI_X11(15*8);         \
	VINSERTI128 $1, X11, Y12, Y12;  \
	VMOVQ_SI_X13(8*8);              \
	VMOVQ_SI_X11(2*8);              \
	VPINSRQ_1_SI_X13_0;             \
	VPINSRQ_1_SI_X11(13*8);         \
	VINSERTI128 $1, X11, Y13, Y13;  \
	LOAD_MSG_AVX2_Y14(10, 3, 7, 9); \
	LOAD_MSG_AVX2_Y15(14, 6, 1, 4)

#define LOAD_MSG_AVX2_7_3_13_11_9_1_12_14_2_5_4_15_6_10_0_8() \
	LOAD_MSG_AVX2_Y12(7, 3, 13, 11); \
	LOAD_MSG_AVX2_Y13(9, 1, 12, 14); \
	LOAD_MSG_AVX2_Y14(2, 5, 4, 15);  \
	VMOVQ_SI_X15(6*8);               \
	VMOVQ_SI_X11_0;                  \
	VPINSRQ_1_SI_X15(10*8);          \
	VPINSRQ_1_SI_X11(8*8);           \
	VINSERTI128 $1, X11, Y15, Y15

#define LOAD_MSG_AVX2_9_5_2_10_0_7_4_15_14_11_6_3_1_12_8_13() \
	LOAD_MSG_AVX2_Y12(9, 5, 2, 10);  \
	VMOVQ_SI_X13_0;                  \
	VMOVQ_SI_X11(4*8);               \
	VPINSRQ_1_SI_X13(7*8);           \
	VPINSRQ_1_SI_X11(15*8);          \
	VINSERTI128 $1, X11, Y13, Y13;   \
	LOAD_MSG_AVX2_Y14(14, 11, 6, 3); \
	LOAD_MSG_AVX2_Y15(1, 12, 8, 13)

#define LOAD_MSG_AVX2_2_6_0_8_12_10_11_3_4_7_15_1_13_5_14_9() \
	VMOVQ_SI_X12(2*8);                \
	VMOVQ_SI_X11_0;                   \
	VPINSRQ_1_SI_X12(6*8);            \
	VPINSRQ_1_SI_X11(8*8);            \
	VINSERTI128 $1, X11, Y12, Y12;    \
	LOAD_MSG_AVX2_Y13(12, 10, 11, 3); \
	LOAD_MSG_AVX2_Y14(4, 7, 15, 1);   \
	LOAD_MSG_AVX2_Y15(13, 5, 14, 9)

#define LOAD_MSG_AVX2_12_1_14_4_5_15_13_10_0_6_9_8_7_3_2_11() \
	LOAD_MSG_AVX2_Y12(12, 1, 14, 4);  \
	LOAD_MSG_AVX2_Y13(5, 15, 13, 10); \
	VMOVQ_SI_X14_0;                   \
	VPSHUFD     $0x4E, 8*8(SI), X11;  \
	VPINSRQ_1_SI_X14(6*8);            \
	VINSERTI128 $1, X11, Y14, Y14;    \
	LOAD_MSG_AVX2_Y15(7, 3, 2, 11)

#define LOAD_MSG_AVX2_13_7_12_3_11_14_1_9_5_15_8_2_0_4_6_10() \
	LOAD_MSG_AVX2_Y12(13, 7, 12, 3); \
	LOAD_MSG_AVX2_Y13(11, 14, 1, 9); \
	LOAD_MSG_AVX2_Y14(5, 15, 8, 2);  \
	VMOVQ_SI_X15_0;                  \
	VMOVQ_SI_X11(6*8);               \
	VPINSRQ_1_SI_X15(4*8);           \
	VPINSRQ_1_SI_X11(10*8);          \
	VINSERTI128 $1, X11, Y15, Y15

#define LOAD_MSG_AVX2_6_14_11_0_15_9_3_8_12_13_1_10_2_7_4_5() \
	VMOVQ_SI_X12(6*8);              \
	VMOVQ_SI_X11(11*8);             \
	VPINSRQ_1_SI_X12(14*8);         \
	VPINSRQ_1_SI_X11_0;             \
	VINSERTI128 $1, X11, Y12, Y12;  \
	LOAD_MSG_AVX2_Y13(15, 9, 3, 8); \
	VMOVQ_SI_X11(1*8);              \
	VMOVDQU     12*8(SI), X14;      \
	VPINSRQ_1_SI_X11(10*8);         \
	VINSERTI128 $1, X11, Y14, Y14;  \
	VMOVQ_SI_X15(2*8);              \
	VMOVDQU     4*8(SI), X11;       \
	VPINSRQ_1_SI_X15(7*8);          \
	VINSERTI128 $1, X11, Y15, Y15

#define LOAD_MSG_AVX2_10_8_7_1_2_4_6_5_15_9_3_13_11_14_12_0() \
	LOAD_MSG_AVX2_Y12(10, 8, 7, 1);  \
	VMOVQ_SI_X13(2*8);               \
	VPSHUFD     $0x4E, 5*8(SI), X11; \
	VPINSRQ_1_SI_X13(4*8);           \
	VINSERTI128 $1, X11, Y13, Y13;   \
	LOAD_MSG_AVX2_Y14(15, 9, 3, 13); \
	VMOVQ_SI_X15(11*8);              \
	VMOVQ_SI_X11(12*8);              \
	VPINSRQ_1_SI_X15(14*8);          \
	VPINSRQ_1_SI_X11_0;              \
	VINSERTI128 $1, X11, Y15, Y15

// func hashBlocksAVX2(h *[8]uint64, c *[2]uint64, flag uint64, blocks []byte)
TEXT ·hashBlocksAVX2(SB), 4, $320-48 // frame size = 288 + 32 byte alignment
	MOVQ h+0(FP), AX
	MOVQ c+8(FP), BX
	MOVQ flag+16(FP), CX
	MOVQ blocks_base+24(FP), SI
	MOVQ blocks_len+32(FP), DI

	MOVQ SP, DX
	ADDQ $31, DX
	ANDQ $~31, DX

	MOVQ CX, 16(DX)
	XORQ CX, CX
	MOVQ CX, 24(DX)

	VMOVDQU ·AVX2_c40<>(SB), Y4
	VMOVDQU ·AVX2_c48<>(SB), Y5

	VMOVDQU 0(AX), Y8
	VMOVDQU 32(AX), Y9
	VMOVDQU ·AVX2_iv0<>(SB), Y6
	VMOVDQU ·AVX2_iv1<>(SB), Y7

	MOVQ 0(BX), R8
	MOVQ 8(BX), R9
	MOVQ R9, 8(DX)

loop:
	ADDQ $128, R8
	MOVQ R8, 0(DX)
	CMPQ R8, $128
	JGE  noinc
	INCQ R9
	MOVQ R9, 8(DX)

noinc:
	VMOVDQA Y8, Y0
	VMOVDQA Y9, Y1
	VMOVDQA Y6, Y2
	VPXOR   0(DX), Y7, Y3

	LOAD_MSG_AVX2_0_2_4_6_1_3_5_7_8_10_12_14_9_11_13_15()
	VMOVDQA Y12, 32(DX)
	VMOVDQA Y13, 64(DX)
	VMOVDQA Y14, 96(DX)
	VMOVDQA Y15, 128(DX)
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_14_4_9_13_10_8_15_6_1_0_11_5_12_2_7_3()
	VMOVDQA Y12, 160(DX)
	VMOVDQA Y13, 192(DX)
	VMOVDQA Y14, 224(DX)
	VMOVDQA Y15, 256(DX)

	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_11_12_5_15_8_0_2_13_10_3_7_9_14_6_1_4()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_7_3_13_11_9_1_12_14_2_5_4_15_6_10_0_8()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_9_5_2_10_0_7_4_15_14_11_6_3_1_12_8_13()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_2_6_0_8_12_10_11_3_4_7_15_1_13_5_14_9()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_12_1_14_4_5_15_13_10_0_6_9_8_7_3_2_11()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_13_7_12_3_11_14_1_9_5_15_8_2_0_4_6_10()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_6_14_11_0_15_9_3_8_12_13_1_10_2_7_4_5()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)
	LOAD_MSG_AVX2_10_8_7_1_2_4_6_5_15_9_3_13_11_14_12_0()
	ROUND_AVX2(Y12, Y13, Y14, Y15, Y10, Y4, Y5)

	ROUND_AVX2(32(DX), 64(DX), 96(DX), 128(DX), Y10, Y4, Y5)
	ROUND_AVX2(160(DX), 192(DX), 224(DX), 256(DX), Y10, Y4, Y5)

	VPXOR Y0, Y8, Y8
	VPXOR Y1, Y9, Y9
	VPXOR Y2, Y8, Y8
	VPXOR Y3, Y9, Y9

	LEAQ 128(SI), SI
	SUBQ $128, DI
	JNE  loop

	MOVQ R8, 0(BX)
	MOVQ R9, 8(BX)

	VMOVDQU Y8, 0(AX)
	VMOVDQU Y9, 32(AX)
	VZEROUPPER

	RET

#define VPUNPCKLQDQ_X2_X2_X15 BYTE $0xC5; BYTE $0x69; BYTE $0x6C; BYTE $0xFA
#define VPUNPCKLQDQ_X3_X3_X15 BYTE $0xC5; BYTE $0x61; BYTE $0x6C; BYTE $0xFB
#define VPUNPCKLQDQ_X7_X7_X15 BYTE $0xC5; BYTE $0x41; BYTE $0x6C; BYTE $0xFF
#define VPUNPCKLQDQ_X13_X13_X15 BYTE $0xC4; BYTE $0x41; BYTE $0x11; BYTE $0x6C; BYTE $0xFD
#define VPUNPCKLQDQ_X14_X14_X15 BYTE $0xC4; BYTE $0x41; BYTE $0x09; BYTE $0x6C; BYTE $0xFE

#define VPUNPCKHQDQ_X15_X2_X2 BYTE $0xC4; BYTE $0xC1; BYTE $0x69; BYTE $0x6D; BYTE $0xD7
#define VPUNPCKHQDQ_X15_X3_X3 BYTE $0xC4; BYTE $0xC1; BYTE $0x61; BYTE $0x6D; BYTE $0xDF
#define VPUNPCKHQDQ_X15_X6_X6 BYTE $0xC4; BYTE $0xC1; BYTE $0x49; BYTE $0x6D; BYTE $0xF7
#define VPUNPCKHQDQ_X15_X7_X7 BYTE $0xC4; BYTE $0xC1; BYTE $0x41; BYTE $0x6D; BYTE $0xFF
#define VPUNPCKHQDQ_X15_X3_X2 BYTE $0xC4; BYTE $0xC1; BYTE $0x61; BYTE $0x6D; BYTE $0xD7
#define VPUNPCKHQDQ_X15_X7_X6 BYTE $0xC4; BYTE $0xC1; BYTE $0x41; BYTE $0x6D; BYTE $0xF7
#define VPUNPCKHQDQ_X15_X13_X3 BYTE $0xC4; BYTE $0xC1; BYTE $0x11; BYTE $0x6D; BYTE $0xDF
#define VPUNPCKHQDQ_X15_X13_X7 BYTE $0xC4; BYTE $0xC1; BYTE $0x11; BYTE $0x6D; BYTE $0xFF

#define SHUFFLE_AVX() \
	VMOVDQA X6, X13;         \
	VMOVDQA X2, X14;         \
	VMOVDQA X4, X6;          \
	VPUNPCKLQDQ_X13_X13_X15; \
	VMOVDQA X5, X4;          \
	VMOVDQA X6, X5;          \
	VPUNPCKHQDQ_X15_X7_X6;   \
	VPUNPCKLQDQ_X7_X7_X15;   \
	VPUNPCKHQDQ_X15_X13_X7;  \
	VPUNPCKLQDQ_X3_X3_X15;   \
	VPUNPCKHQDQ_X15_X2_X2;   \
	VPUNPCKLQDQ_X14_X14_X15; \
	VPUNPCKHQDQ_X15_X3_X3;   \

#define SHUFFLE_AVX_INV() \
	VMOVDQA X2, X13;         \
	VMOVDQA X4, X14;         \
	VPUNPCKLQDQ_X2_X2_X15;   \
	VMOVDQA X5, X4;          \
	VPUNPCKHQDQ_X15_X3_X2;   \
	VMOVDQA X14, X5;         \
	VPUNPCKLQDQ_X3_X3_X15;   \
	VMOVDQA X6, X14;         \
	VPUNPCKHQDQ_X15_X13_X3;  \
	VPUNPCKLQDQ_X7_X7_X15;   \
	VPUNPCKHQDQ_X15_X6_X6;   \
	VPUNPCKLQDQ_X14_X14_X15; \
	VPUNPCKHQDQ_X15_X7_X7;   \

#define HALF_ROUND_AVX(v0, v1, v2, v3, v4, v5, v6, v7, m0, m1, m2, m3, t0, c40, c48) \
	VPADDQ  m0, v0, v0;   \
	VPADDQ  v2, v0, v0;   \
	VPADDQ  m1, v1, v1;   \
	VPADDQ  v3, v1, v1;   \
	VPXOR   v0, v6, v6;   \
	VPXOR   v1, v7, v7;   \
	VPSHUFD $-79, v6, v6; \
	VPSHUFD $-79, v7, v7; \
	VPADDQ  v6, v4, v4;   \
	VPADDQ  v7, v5, v5;   \
	VPXOR   v4, v2, v2;   \
	VPXOR   v5, v3, v3;   \
	VPSHUFB c40, v2, v2;  \
	VPSHUFB c40, v3, v3;  \
	VPADDQ  m2, v0, v0;   \
	VPADDQ  v2, v0, v0;   \
	VPADDQ  m3, v1, v1;   \
	VPADDQ  v3, v1, v1;   \
	VPXOR   v0, v6, v6;   \
	VPXOR   v1, v7, v7;   \
	VPSHUFB c48, v6, v6;  \
	VPSHUFB c48, v7, v7;  \
	VPADDQ  v6, v4, v4;   \
	VPADDQ  v7, v5, v5;   \
	VPXOR   v4, v2, v2;   \
	VPXOR   v5, v3, v3;   \
	VPADDQ  v2, v2, t0;   \
	VPSRLQ  $63, v2, v2;  \
	VPXOR   t0, v2, v2;   \
	VPADDQ  v3, v3, t0;   \
	VPSRLQ  $63, v3, v3;  \
	VPXOR   t0, v3, v3

// load msg: X12 = (i0, i1), X13 = (i2, i3), X14 = (i4, i5), X15 = (i6, i7)
// i0, i1, i2, i3, i4, i5, i6, i7 must not be 0
#define LOAD_MSG_AVX(i0, i1, i2, i3, i4, i5, i6, i7) \
	VMOVQ_SI_X12(i0*8);     \
	VMOVQ_SI_X13(i2*8);     \
	VMOVQ_SI_X14(i4*8);     \
	VMOVQ_SI_X15(i6*8);     \
	VPINSRQ_1_SI_X12(i1*8); \
	VPINSRQ_1_SI_X13(i3*8); \
	VPINSRQ_1_SI_X14(i5*8); \
	VPINSRQ_1_SI_X15(i7*8)

// load msg: X12 = (0, 2), X13 = (4, 6), X14 = (1, 3), X15 = (5, 7)
#define LOAD_MSG_AVX_0_2_4_6_1_3_5_7() \
	VMOVQ_SI_X12_0;        \
	VMOVQ_SI_X13(4*8);     \
	VMOVQ_SI_X14(1*8);     \
	VMOVQ_SI_X15(5*8);     \
	VPINSRQ_1_SI_X12(2*8); \
	VPINSRQ_1_SI_X13(6*8); \
	VPINSRQ_1_SI_X14(3*8); \
	VPINSRQ_1_SI_X15(7*8)

// load msg: X12 = (1, 0), X13 = (11, 5), X14 = (12, 2), X15 = (7, 3)
#define LOAD_MSG_AVX_1_0_11_5_12_2_7_3() \
	VPSHUFD $0x4E, 0*8(SI), X12; \
	VMOVQ_SI_X13(11*8);          \
	VMOVQ_SI_X14(12*8);          \
	VMOVQ_SI_X15(7*8);           \
	VPINSRQ_1_SI_X13(5*8);       \
	VPINSRQ_1_SI_X14(2*8);       \
	VPINSRQ_1_SI_X15(3*8)

// load msg: X12 = (11, 12), X13 = (5, 15), X14 = (8, 0), X15 = (2, 13)
#define LOAD_MSG_AVX_11_12_5_15_8_0_2_13() \
	VMOVDQU 11*8(SI), X12;  \
	VMOVQ_SI_X13(5*8);      \
	VMOVQ_SI_X14(8*8);      \
	VMOVQ_SI_X15(2*8);      \
	VPINSRQ_1_SI_X13(15*8); \
	VPINSRQ_1_SI_X14_0;     \
	VPINSRQ_1_SI_X15(13*8)

// load msg: X12 = (2, 5), X13 = (4, 15), X14 = (6, 10), X15 = (0, 8)
#define LOAD_MSG_AVX_2_5_4_15_6_10_0_8() \
	VMOVQ_SI_X12(2*8);      \
	VMOVQ_SI_X13(4*8);      \
	VMOVQ_SI_X14(6*8);      \
	VMOVQ_SI_X15_0;         \
	VPINSRQ_1_SI_X12(5*8);  \
	VPINSRQ_1_SI_X13(15*8); \
	VPINSRQ_1_SI_X14(10*8); \
	VPINSRQ_1_SI_X15(8*8)

// load msg: X12 = (9, 5), X13 = (2, 10), X14 = (0, 7), X15 = (4, 15)
#define LOAD_MSG_AVX_9_5_2_10_0_7_4_15() \
	VMOVQ_SI_X12(9*8);      \
	VMOVQ_SI_X13(2*8);      \
	VMOVQ_SI_X14_0;         \
	VMOVQ_SI_X15(4*8);      \
	VPINSRQ_1_SI_X12(5*8);  \
	VPINSRQ_1_SI_X13(10*8); \
	VPINSRQ_1_SI_X14(7*8);  \
	VPINSRQ_1_SI_X15(15*8)

// load msg: X12 = (2, 6), X13 = (0, 8), X14 = (12, 10), X15 = (11, 3)
#define LOAD_MSG_AVX_2_6_0_8_12_10_11_3() \
	VMOVQ_SI_X12(2*8);      \
	VMOVQ_SI_X13_0;         \
	VMOVQ_SI_X14(12*8);     \
	VMOVQ_SI_X15(11*8);     \
	VPINSRQ_1_SI_X12(6*8);  \
	VPINSRQ_1_SI_X13(8*8);  \
	VPINSRQ_1_SI_X14(10*8); \
	VPINSRQ_1_SI_X15(3*8)

// load msg: X12 = (0, 6), X13 = (9, 8), X14 = (7, 3), X15 = (2, 11)
#define LOAD_MSG_AVX_0_6_9_8_7_3_2_11() \
	MOVQ    0*8(SI), X12;        \
	VPSHUFD $0x4E, 8*8(SI), X13; \
	MOVQ    7*8(SI), X14;        \
	MOVQ    2*8(SI), X15;        \
	VPINSRQ_1_SI_X12(6*8);       \
	VPINSRQ_1_SI_X14(3*8);       \
	VPINSRQ_1_SI_X15(11*8)

// load msg: X12 = (6, 14), X13 = (11, 0), X14 = (15, 9), X15 = (3, 8)
#define LOAD_MSG_AVX_6_14_11_0_15_9_3_8() \
	MOVQ 6*8(SI), X12;      \
	MOVQ 11*8(SI), X13;     \
	MOVQ 15*8(SI), X14;     \
	MOVQ 3*8(SI), X15;      \
	VPINSRQ_1_SI_X12(14*8); \
	VPINSRQ_1_SI_X13_0;     \
	VPINSRQ_1_SI_X14(9*8);  \
	VPINSRQ_1_SI_X15(8*8)

// load msg: X12 = (5, 15), X13 = (8, 2), X14 = (0, 4), X15 = (6, 10)
#define LOAD_MSG_AVX_5_15_8_2_0_4_6_10() \
	MOVQ 5*8(SI), X12;      \
	MOVQ 8*8(SI), X13;      \
	MOVQ 0*8(SI), X14;      \
	MOVQ 6*8(SI), X15;      \
	VPINSRQ_1_SI_X12(15*8); \
	VPINSRQ_1_SI_X13(2*8);  \
	VPINSRQ_1_SI_X14(4*8);  \
	VPINSRQ_1_SI_X15(10*8)

// load msg: X12 = (12, 13), X13 = (1, 10), X14 = (2, 7), X15 = (4, 5)
#define LOAD_MSG_AVX_12_13_1_10_2_7_4_5() \
	VMOVDQU 12*8(SI), X12;  \
	MOVQ    1*8(SI), X13;   \
	MOVQ    2*8(SI), X14;   \
	VPINSRQ_1_SI_X13(10*8); \
	VPINSRQ_1_SI_X14(7*8);  \
	VMOVDQU 4*8(SI), X15

// load msg: X12 = (15, 9), X13 = (3, 13), X14 = (11, 14), X15 = (12, 0)
#define LOAD_MSG_AVX_15_9_3_13_11_14_12_0() \
	MOVQ 15*8(SI), X12;     \
	MOVQ 3*8(SI), X13;      \
	MOVQ 11*8(SI), X14;     \
	MOVQ 12*8(SI), X15;     \
	VPINSRQ_1_SI_X12(9*8);  \
	VPINSRQ_1_SI_X13(13*8); \
	VPINSRQ_1_SI_X14(14*8); \
	VPINSRQ_1_SI_X15_0

// func hashBlocksAVX(h *[8]uint64, c *[2]uint64, flag uint64, blocks []byte)
TEXT ·hashBlocksAVX(SB), 4, $288-48 // frame size = 272 + 16 byte alignment
	MOVQ h+0(FP), AX
	MOVQ c+8(FP), BX
	MOVQ flag+16(FP), CX
	MOVQ blocks_base+24(FP), SI
	MOVQ blocks_len+32(FP), DI

	MOVQ SP, R10
	ADDQ $15, R10
	ANDQ $~15, R10

	VMOVDQU ·AVX_c40<>(SB), X0
	VMOVDQU ·AVX_c48<>(SB), X1
	VMOVDQA X0, X8
	VMOVDQA X1, X9

	VMOVDQU ·AVX_iv3<>(SB), X0
	VMOVDQA X0, 0(R10)
	XORQ    CX, 0(R10)          // 0(R10) = ·AVX_iv3 ^ (CX || 0)

	VMOVDQU 0(AX), X10
	VMOVDQU 16(AX), X11
	VMOVDQU 32(AX), X2
	VMOVDQU 48(AX), X3

	MOVQ 0(BX), R8
	MOVQ 8(BX), R9

loop:
	ADDQ $128, R8
	CMPQ R8, $128
	JGE  noinc
	INCQ R9

noinc:
	VMOVQ_R8_X15
	VPINSRQ_1_R9_X15

	VMOVDQA X10, X0
	VMOVDQA X11, X1
	VMOVDQU ·AVX_iv0<>(SB), X4
	VMOVDQU ·AVX_iv1<>(SB), X5
	VMOVDQU ·AVX_iv2<>(SB), X6

	VPXOR   X15, X6, X6
	VMOVDQA 0(R10), X7

	LOAD_MSG_AVX_0_2_4_6_1_3_5_7()
	VMOVDQA X12, 16(R10)
	VMOVDQA X13, 32(R10)
	VMOVDQA X14, 48(R10)
	VMOVDQA X15, 64(R10)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX(8, 10, 12, 14, 9, 11, 13, 15)
	VMOVDQA X12, 80(R10)
	VMOVDQA X13, 96(R10)
	VMOVDQA X14, 112(R10)
	VMOVDQA X15, 128(R10)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX(14, 4, 9, 13, 10, 8, 15, 6)
	VMOVDQA X12, 144(R10)
	VMOVDQA X13, 160(R10)
	VMOVDQA X14, 176(R10)
	VMOVDQA X15, 192(R10)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_1_0_11_5_12_2_7_3()
	VMOVDQA X12, 208(R10)
	VMOVDQA X13, 224(R10)
	VMOVDQA X14, 240(R10)
	VMOVDQA X15, 256(R10)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX_11_12_5_15_8_0_2_13()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX(10, 3, 7, 9, 14, 6, 1, 4)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX(7, 3, 13, 11, 9, 1, 12, 14)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_2_5_4_15_6_10_0_8()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX_9_5_2_10_0_7_4_15()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX(14, 11, 6, 3, 1, 12, 8, 13)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX_2_6_0_8_12_10_11_3()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX(4, 7, 15, 1, 13, 5, 14, 9)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX(12, 1, 14, 4, 5, 15, 13, 10)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_0_6_9_8_7_3_2_11()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX(13, 7, 12, 3, 11, 14, 1, 9)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_5_15_8_2_0_4_6_10()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX_6_14_11_0_15_9_3_8()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_12_13_1_10_2_7_4_5()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	LOAD_MSG_AVX(10, 8, 7, 1, 2, 4, 6, 5)
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX()
	LOAD_MSG_AVX_15_9_3_13_11_14_12_0()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, X12, X13, X14, X15, X15, X8, X9)
	SHUFFLE_AVX_INV()

	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, 16(R10), 32(R10), 48(R10), 64(R10), X15, X8, X9)
	SHUFFLE_AVX()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, 80(R10), 96(R10), 112(R10), 128(R10), X15, X8, X9)
	SHUFFLE_AVX_INV()

	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, 144(R10), 160(R10), 176(R10), 192(R10), X15, X8, X9)
	SHUFFLE_AVX()
	HALF_ROUND_AVX(X0, X1, X2, X3, X4, X5, X6, X7, 208(R10), 224(R10), 240(R10), 256(R10), X15, X8, X9)
	SHUFFLE_AVX_INV()

	VMOVDQU 32(AX), X14
	VMOVDQU 48(AX), X15
	VPXOR   X0, X10, X10
	VPXOR   X1, X11, X11
	VPXOR   X2, X14, X14
	VPXOR   X3, X15, X15
	VPXOR   X4, X10, X10
	VPXOR   X5, X11, X11
	VPXOR   X6, X14, X2
	VPXOR   X7, X15, X3
	VMOVDQU X2, 32(AX)
	VMOVDQU X3, 48(AX)

	LEAQ 128(SI), SI
	SUBQ $128, DI
	JNE  loop

	VMOVDQU X10, 0(AX)
	VMOVDQU X11, 16(AX)

	MOVQ R8, 0(BX)
	MOVQ R9, 8(BX)
	VZEROUPPER

	RET
