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

// TODO: cpuid for AVX? Or can we do this in SSE?

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
TEXT ·accumulateSIMD(SB), NOSPLIT, $8-48
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ src_len+32(FP), R9

	// TODO: clean up the tail if len(src)%4 != 0.
	ANDQ $-4, R9

	// AX holds the variable i.
	MOVQ $0, AX

	// twoFiftyFives := XMM(0x437f0000 repeated four times) // 255 as a float32.
	// ones          := XMM(0x3f800000 repeated four times) // 1 as a float32.
	// signMask      := XMM(0x80000000 repeated four times) // The sign bit of a float32.
	// mask          := XMM(0x0c080400 repeated four times) // Shuffle mask.
	//
	// movl etc
	// vbroadcastss (%rsp),%xmm3
	// movl etc
	// vbroadcastss (%rsp),%xmm4
	// movl etc
	// vbroadcastss (%rsp),%xmm5
	// movl etc
	// vbroadcastss (%rsp),%xmm6
	//
	// Note that broadcast-8(SP) is an offset (-8) to a pseudo-register. The
	// offset to the physical register is 0. See https://golang.org/doc/asm and
	// "The SP pseudo-register is a virtual stack pointer...".
	MOVL $0x437f0000, broadcast-8(SP)
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x1c; BYTE $0x24
	MOVL $0x3f800000, broadcast-8(SP)
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x24; BYTE $0x24
	MOVL $0x80000000, broadcast-8(SP)
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x2c; BYTE $0x24
	MOVL $0x0c080400, broadcast-8(SP)
	BYTE $0xc4; BYTE $0xe2; BYTE $0x79; BYTE $0x18; BYTE $0x34; BYTE $0x24

	// offset = XMM(0, 0, 0, 0)
	//
	// vxorps %xmm7,%xmm7,%xmm7
	BYTE $0xc5; BYTE $0xc0; BYTE $0x57; BYTE $0xff

loop:
	// for i < len(src)
	CMPQ AX, R9
	JAE  end

	// x = XMM(s0, s1, s2, s3)
	//
	// Where s0 is src[0], s1 is src[1], etc.
	//
	// vmovups (%rsi),%xmm1
	BYTE $0xc5; BYTE $0xf8; BYTE $0x10; BYTE $0x0e

	// scratch = XMM(0, s0, s1, s2)
	// x += scratch // yields x == XMM(s0, s0+s1, s1+s2, s2+s3)
	//
	// vmovaps %xmm1,%xmm0
	// vpslldq $0x4,%xmm0,%xmm0
	// vaddps %xmm0,%xmm1,%xmm1
	BYTE $0xc5; BYTE $0xf8; BYTE $0x28; BYTE $0xc1
	BYTE $0xc5; BYTE $0xf9; BYTE $0x73; BYTE $0xf8; BYTE $0x04
	BYTE $0xc5; BYTE $0xf0; BYTE $0x58; BYTE $0xc8

	// scratch = XMM(0, 0, 0, 0)
	// scratch = XMM(scratch@0, scratch@0, x@0, x@1) // yields scratch == XMM(0, 0, s0, s0+s1)
	// x += scratch // yields x == XMM(s0, s0+s1, s0+s1+s2, s0+s1+s2+s3)
	//
	// vxorps %xmm0,%xmm0,%xmm0
	// vshufps $0x40,%xmm1,%xmm0,%xmm0
	// vaddps %xmm0,%xmm1,%xmm1
	BYTE $0xc5; BYTE $0xf8; BYTE $0x57; BYTE $0xc0
	BYTE $0xc5; BYTE $0xf8; BYTE $0xc6; BYTE $0xc1; BYTE $0x40
	BYTE $0xc5; BYTE $0xf0; BYTE $0x58; BYTE $0xc8

	// x += offset
	//
	// vaddps %xmm7,%xmm1,%xmm1
	BYTE $0xc5; BYTE $0xf0; BYTE $0x58; BYTE $0xcf

	// y = x &^ signMask
	// y = min(y, ones)
	// y = mul(y, twoFiftyFives)
	//
	// vandnps %xmm1,%xmm5,%xmm2
	// vminps %xmm4,%xmm2,%xmm2
	// vmulps %xmm3,%xmm2,%xmm2
	BYTE $0xc5; BYTE $0xd0; BYTE $0x55; BYTE $0xd1
	BYTE $0xc5; BYTE $0xe8; BYTE $0x5d; BYTE $0xd4
	BYTE $0xc5; BYTE $0xe8; BYTE $0x59; BYTE $0xd3

	// z = float32ToInt32(y)
	// z = shuffleTheLowBytesOfEach4ByteElement(z)
	// copy(dst[:4], low4BytesOf(z))
	//
	// vcvtps2dq %xmm2,%xmm2
	// vpshufb %xmm6,%xmm2,%xmm2
	// vmovd %xmm2,(%rdi)
	BYTE $0xc5; BYTE $0xf9; BYTE $0x5b; BYTE $0xd2
	BYTE $0xc4; BYTE $0xe2; BYTE $0x69; BYTE $0x00; BYTE $0xd6
	BYTE $0xc5; BYTE $0xf9; BYTE $0x7e; BYTE $0x17

	// offset = XMM(x@3, x@3, x@3, x@3)
	//
	// vshufps $0xff,%xmm1,%xmm1,%xmm7
	BYTE $0xc5; BYTE $0xf0; BYTE $0xc6; BYTE $0xf9; BYTE $0xff

	// i += 4
	// dst = dst[4:]
	// src = src[4:]
	ADDQ $4, AX
	ADDQ $4, DI
	ADDQ $16, SI
	JMP  loop

end:
	RET
