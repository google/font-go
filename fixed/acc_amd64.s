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

DATA effEffEffs<>+0x00(SB)/8, $0x00000fff00000fff
DATA effEffEffs<>+0x08(SB)/8, $0x00000fff00000fff
DATA mask<>+0x00(SB)/8, $0x0c0804000c080400
DATA mask<>+0x08(SB)/8, $0x0c0804000c080400

GLOBL effEffEffs<>(SB), (NOPTR+RODATA), $16
GLOBL mask<>(SB), (NOPTR+RODATA), $16

// func haveSSE4_1() bool
TEXT ·haveSSE4_1(SB), NOSPLIT, $0
	MOVQ $1, AX
	CPUID
	SHRQ $19, CX
	ANDQ $1, CX
	MOVB CX, ret+0(FP)
	RET

// func accumulateSIMD(dst []uint8, src []int20_12)
//
// XMM registers. Names are per
// https://github.com/google/font-rs/blob/master/src/accumulate.c
//
//	xmm0	scratch
//	xmm1	x
//	xmm2	y, z
//	xmm3	-
//	xmm4	-
//	xmm5	effEffEffs
//	xmm6	mask
//	xmm7	offset
TEXT ·accumulateSIMD(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI
	MOVQ dst_len+8(FP), BX
	MOVQ src_base+24(FP), SI
	MOVQ src_len+32(FP), CX

	// Sanity check that len(dst) >= len(src).
	CMPQ BX, CX
	JLT  end

	// CX = len(src) &^ 3
	// DX = len(src)
	MOVQ CX, DX
	ANDQ $-4, CX

	// effEffEffs    := XMM(0x00000fff repeated four times) // Effective maximum of an int20_12.
	// mask          := XMM(0x0c080400 repeated four times) // Shuffle mask.
	// offset        := XMM(0x00000000 repeated four times) // Cumulative sum.
	MOVOU effEffEffs<>(SB), X5
	MOVOU mask<>(SB), X6
	XORPS X7, X7

	// i := 0
	MOVQ $0, AX

loop4:
	// for i < (len(src) &^ 3)
	CMPQ AX, CX
	JAE  loop1

	// x = XMM(s0, s1, s2, s3)
	//
	// Where s0 is src[i+0], s1 is src[i+1], etc.
	MOVOU (SI), X1

	// scratch = XMM(0, s0, s1, s2)
	// x += scratch // yields x == XMM(s0, s0+s1, s1+s2, s2+s3)
	MOVOU X1, X0
	PSLLO $4, X0
	PADDD X0, X1

	// scratch = XMM(0, 0, 0, 0)
	// scratch = XMM(scratch@0, scratch@0, x@0, x@1) // yields scratch == XMM(0, 0, s0, s0+s1)
	// x += scratch // yields x == XMM(s0, s0+s1, s0+s1+s2, s0+s1+s2+s3)
	XORPS  X0, X0
	SHUFPS $0x40, X1, X0
	PADDD  X0, X1

	// x += offset
	PADDD X7, X1

	// y = abs(x)
	// y = min(y, effEffEffs)
	// y >>= 4
	//
	// pabsd  %xmm1,%xmm2
	// pminud %xmm5,%xmm2
	// psrld  $0x4,%xmm2
	BYTE $0x66; BYTE $0x0f; BYTE $0x38; BYTE $0x1e; BYTE $0xd1
	BYTE $0x66; BYTE $0x0f; BYTE $0x38; BYTE $0x3b; BYTE $0xd5
	BYTE $0x66; BYTE $0x0f; BYTE $0x72; BYTE $0xd2; BYTE $0x04

	// z = shuffleTheLowBytesOfEach4ByteElement(y)
	// copy(dst[:4], low4BytesOf(z))
	PSHUFB X6, X2
	MOVL   X2, (DI)

	// offset = XMM(x@3, x@3, x@3, x@3)
	MOVOU  X1, X7
	SHUFPS $0xff, X1, X7

	// i += 4
	// dst = dst[4:]
	// src = src[4:]
	ADDQ $4, AX
	ADDQ $4, DI
	ADDQ $16, SI
	JMP  loop4

loop1:
	// for i < len(src)
	CMPQ AX, DX
	JAE  end

	// x = src[i] + offset
	MOVL  (SI), X1
	PADDD X7, X1

	// y = abs(x)
	// y = min(y, effEffEffs)
	// y >>= 4
	//
	// pabsd  %xmm1,%xmm2
	// pminud %xmm5,%xmm2
	// psrld  $0x4,%xmm2
	BYTE $0x66; BYTE $0x0f; BYTE $0x38; BYTE $0x1e; BYTE $0xd1
	BYTE $0x66; BYTE $0x0f; BYTE $0x38; BYTE $0x3b; BYTE $0xd5
	BYTE $0x66; BYTE $0x0f; BYTE $0x72; BYTE $0xd2; BYTE $0x04

	// dst[0] = uint8(y)
	MOVL X2, BX
	MOVB BX, (DI)

	// offset = x
	MOVOU X1, X7

	// i += 1
	// dst = dst[1:]
	// src = src[1:]
	ADDQ $1, AX
	ADDQ $1, DI
	ADDQ $4, SI
	JMP  loop1

end:
	RET
