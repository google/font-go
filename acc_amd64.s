// Copyright 2016 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !appengine
// +build gc
// +build !noasm

#include "textflag.h"

DATA twoFiftyFives<>+0x00(SB)/8, $0x437f0000437f0000
DATA twoFiftyFives<>+0x08(SB)/8, $0x437f0000437f0000
DATA ones<>+0x00(SB)/8, $0x3f8000003f800000
DATA ones<>+0x08(SB)/8, $0x3f8000003f800000
DATA signMask<>+0x00(SB)/8, $0x7fffffff7fffffff
DATA signMask<>+0x08(SB)/8, $0x7fffffff7fffffff
DATA mask<>+0x00(SB)/8, $0x0c0804000c080400
DATA mask<>+0x08(SB)/8, $0x0c0804000c080400

GLOBL twoFiftyFives<>(SB), (NOPTR+RODATA), $16
GLOBL ones<>(SB), (NOPTR+RODATA), $16
GLOBL signMask<>(SB), (NOPTR+RODATA), $16
GLOBL mask<>(SB), (NOPTR+RODATA), $16

// func accumulateSIMD(dst []uint8, src []float32)
//
// XMM registers. Names are per
// https://github.com/google/font-rs/blob/master/src/accumulate.c
//
//	xmm0	scratch
//	xmm1	x
//	xmm2	y, z
//	xmm3	twoFiftyFives
//	xmm4	ones
//	xmm5	signMask
//	xmm6	mask
//	xmm7	offset
TEXT ·accumulateSIMD(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ src_len+32(FP), R9

	// TODO: clean up the tail if len(src)%4 != 0.
	ANDQ $-4, R9

	// twoFiftyFives := XMM(0x437f0000 repeated four times) // 255 as a float32.
	// ones          := XMM(0x3f800000 repeated four times) // 1 as a float32.
	// signMask      := XMM(0x7fffffff repeated four times) // All but the sign bit of a float32.
	// mask          := XMM(0x0c080400 repeated four times) // Shuffle mask.
	// offset        := XMM(0x00000000 repeated four times) // Cumulative sum.
	MOVOU twoFiftyFives<>(SB), X3
	MOVOU ones<>(SB), X4
	MOVOU signMask<>(SB), X5
	MOVOU mask<>(SB), X6
	XORPS X7, X7

	// i := 0
	MOVQ $0, AX

loop:
	// for i < len(src)
	CMPQ AX, R9
	JAE  end

	// x = XMM(s0, s1, s2, s3)
	//
	// Where s0 is src[0], s1 is src[1], etc.
	MOVOU (SI), X1

	// scratch = XMM(0, s0, s1, s2)
	// x += scratch // yields x == XMM(s0, s0+s1, s1+s2, s2+s3)
	MOVOU X1, X0
	PSLLO $4, X0
	ADDPS X0, X1

	// scratch = XMM(0, 0, 0, 0)
	// scratch = XMM(scratch@0, scratch@0, x@0, x@1) // yields scratch == XMM(0, 0, s0, s0+s1)
	// x += scratch // yields x == XMM(s0, s0+s1, s0+s1+s2, s0+s1+s2+s3)
	XORPS  X0, X0
	SHUFPS $0x40, X1, X0
	ADDPS  X0, X1

	// x += offset
	ADDPS X7, X1

	// y = x & signMask
	// y = min(y, ones)
	// y = mul(y, twoFiftyFives)
	MOVOU X5, X2
	ANDPS X1, X2
	MINPS X4, X2
	MULPS X3, X2

	// z = float32ToInt32(y)
	// z = shuffleTheLowBytesOfEach4ByteElement(z)
	// copy(dst[:4], low4BytesOf(z))
	CVTPS2PL X2, X2
	PSHUFB   X6, X2
	MOVL     X2, (DI)

	// offset = XMM(x@3, x@3, x@3, x@3)
	MOVOU  X1, X7
	SHUFPS $0xff, X1, X7

	// i += 4
	// dst = dst[4:]
	// src = src[4:]
	ADDQ $4, AX
	ADDQ $4, DI
	ADDQ $16, SI
	JMP  loop

end:
	RET
