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

package main

import (
	"image"
	"io/ioutil"
	"math"
	"testing"
)

func TestAccumulateSIMDUnaligned(t *testing.T) {
	if !haveAccumulateSIMD {
		t.Skip("No accumulateSIMD implemention")
	}

	dst := make([]uint8, 64)
	src := make([]int2ϕ, 64)

	for d := 0; d < 16; d++ {
		for s := 0; s < 16; s++ {
			accumulateSIMD(dst[d:d+32], src[s:s+32])
		}
	}
}

func TestAccumulateSIMDShortDst(t *testing.T) {
	if !haveAccumulateSIMD {
		t.Skip("No accumulateSIMD implemention")
	}

	const oneQuarter = 1 << (2*ϕ - 2)
	dst := make([]uint8, 4)
	src := []int2ϕ{oneQuarter, oneQuarter, oneQuarter, oneQuarter}
	accumulateSIMD(dst[:0], src)
	for i, got := range dst {
		if got != 0 {
			t.Errorf("i=%d: got %#02x, want %#02x", i, got, 0)
		}
	}
}

func TestAccumulate(t *testing.T)              { testAccumulate(t, sequence, sequenceAcc, false) }
func TestAccumulateSIMD(t *testing.T)          { testAccumulate(t, sequence, sequenceAcc, true) }
func TestAccumulateRobotoG16(t *testing.T)     { testAccumulate(t, robotoG16, robotoG16Acc, false) }
func TestAccumulateSIMDRobotoG16(t *testing.T) { testAccumulate(t, robotoG16, robotoG16Acc, true) }

func BenchmarkAccumulate16(b *testing.B)      { benchAccumulate(b, robotoG16, false) }
func BenchmarkAccumulateSIMD16(b *testing.B)  { benchAccumulate(b, robotoG16, true) }
func BenchmarkAccumulate100(b *testing.B)     { benchAccumulate(b, robotoG100, false) }
func BenchmarkAccumulateSIMD100(b *testing.B) { benchAccumulate(b, robotoG100, true) }

func testAccumulate(t *testing.T, src []int2ϕ, want []byte, simd bool) {
	if simd && !haveAccumulateSIMD {
		t.Skip("No accumulateSIMD implemention")
	}

	for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
		13, 14, 15, 16, 17, 41, 58, 79, 96, len(src)} {

		if n > len(src) {
			continue
		}
		got := make([]byte, n)
		if simd {
			accumulateSIMD(got, src[:n])
		} else {
			accumulate(got, src[:n])
		}

	loop:
		for i := range got {
			g := got[i]
			w := want[i]
			if g != w {
				t.Errorf("n=%d, i=%d: got %#02x, want %#02x", n, i, g, w)
				break loop
			}
		}
	}
}

func benchAccumulate(b *testing.B, src []int2ϕ, simd bool) {
	if simd && !haveAccumulateSIMD {
		b.Skip("No accumulateSIMD implemention")
	}

	dst := make([]byte, len(src))
	acc := accumulate
	if simd {
		acc = accumulateSIMD
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc(dst, src)
	}
}

func TestRasterizePolygon(t *testing.T) {
	t.Skip("TODO: make this test pass")
	for radius := 4; radius <= 1024; radius *= 2 {
		z := newRasterizer(2*radius, 2*radius)
		for n := 3; n <= 17; n++ {
			z.reset()
			z.moveTo(point{
				x: float32(2 * radius),
				y: float32(1 * radius),
			})
			for i := 1; i < n; i++ {
				z.lineTo(point{
					x: float32(float64(radius) * (1 + math.Cos(float64(i)*2*math.Pi/float64(n)))),
					y: float32(float64(radius) * (1 + math.Sin(float64(i)*2*math.Pi/float64(n)))),
				})
			}
			z.closePath()

			dst := image.NewAlpha(z.Bounds())
			if haveAccumulateSIMD {
				accumulateSIMD(dst.Pix, z.a)
			} else {
				accumulate(dst.Pix, z.a)
			}

			corners := [4]uint8{
				dst.Pix[(0*radius+0)*dst.Stride+(0*radius+0)],
				dst.Pix[(0*radius+0)*dst.Stride+(2*radius-1)],
				dst.Pix[(2*radius-1)*dst.Stride+(0*radius+0)],
				dst.Pix[(2*radius-1)*dst.Stride+(2*radius-1)],
			}
			if corners != [4]uint8{} {
				t.Errorf("radius=%d, n=%d: corners were not all zero: %v", radius, n, corners)
				continue
			}
			center := dst.Pix[radius*dst.Stride+radius]
			if center < 0xfe { // TODO: can we tighten this to 0xff?
				t.Errorf("radius=%d, n=%d: center: got %#02x, want >= 0xfe", radius, n, center)
				continue
			}
		}
	}
}

func BenchmarkRasterize16(b *testing.B)  { benchRasterize(b, 16) }
func BenchmarkRasterize32(b *testing.B)  { benchRasterize(b, 32) }
func BenchmarkRasterize64(b *testing.B)  { benchRasterize(b, 64) }
func BenchmarkRasterize100(b *testing.B) { benchRasterize(b, 100) }
func BenchmarkRasterize150(b *testing.B) { benchRasterize(b, 150) }
func BenchmarkRasterize200(b *testing.B) { benchRasterize(b, 200) }

func benchRasterize(b *testing.B, ppem float32) {
	fontData, err := ioutil.ReadFile(*fontFlag)
	if err != nil {
		b.Fatal(err)
	}
	f, err := parse(fontData)
	if err != nil {
		b.Fatal(err)
	}

	data := f.glyphData(uint16(*glyphIDFlag))
	dx, dy, transform := data.glyphSizeAndTransform(f.scale(ppem))
	z := newRasterizer(dx, dy)
	dst := image.NewAlpha(z.Bounds())

	acc := accumulate
	if haveAccumulateSIMD {
		acc = accumulateSIMD
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z.reset()
		z.rasterize(f, data, transform)
		acc(dst.Pix, z.a)
	}
}

// sequenceAcc is the accumulation of sequence.
var sequenceAcc = []uint8{
	0x20,
	0x60,
	0x20,
	0x40,
	0x60,
	0x60,
	0xa0,
	0xff,
	0xe0,
	0x00,
	0x40,
}

var sequence = []int2ϕ{
	+0x020000, // +0.125, // Running sum: +0.125
	-0x080000, // -0.500, // Running sum: -0.375
	+0x040000, // +0.250, // Running sum: -0.125
	+0x060000, // +0.375, // Running sum: +0.250
	+0x020000, // +0.125, // Running sum: +0.375
	+0x000000, // +0.000, // Running sum: +0.375
	-0x100000, // -1.000, // Running sum: -0.625
	-0x080000, // -0.500, // Running sum: -1.125
	+0x040000, // +0.250, // Running sum: -0.875
	+0x0e0000, // +0.875, // Running sum: +0.000
	+0x040000, // +0.250, // Running sum: +0.250
}

// robotoG16Acc is the accumulation of roboto16.
var robotoG16Acc = []uint8{
	0x00, 0x00, 0x27, 0x7b, 0x86, 0x3f, 0x33, 0x67,
	0x00, 0x3e, 0xf3, 0xde, 0xad, 0xe5, 0xd8, 0xe4,
	0x00, 0xcd, 0xcc, 0x0a, 0x00, 0x0e, 0xd2, 0xe4,
	0x18, 0xff, 0x61, 0x00, 0x00, 0x00, 0x90, 0xe4,
	0x36, 0xff, 0x3e, 0x00, 0x00, 0x00, 0x90, 0xe4,
	0x27, 0xff, 0x46, 0x00, 0x00, 0x00, 0x90, 0xe4,
	0x07, 0xf5, 0x83, 0x00, 0x00, 0x00, 0x9b, 0xe4,
	0x00, 0x8e, 0xf4, 0x5a, 0x21, 0x6a, 0xfc, 0xe4,
	0x00, 0x08, 0xa4, 0xfc, 0xff, 0xcb, 0xb3, 0xe3,
	0x00, 0x00, 0x00, 0x0b, 0x13, 0x00, 0xa9, 0xc6,
	0x00, 0x7b, 0x86, 0x09, 0x00, 0x3e, 0xf8, 0x7a,
	0x00, 0x2b, 0xda, 0xfa, 0xe8, 0xff, 0xa3, 0x05,
	0x00, 0x00, 0x01, 0x27, 0x47, 0x20, 0x00, 0x00,
}

// robotoG16 is the to-be-accumulated 'g' from Roboto-Regular.ttf at 16 ppem.
var robotoG16 = []int2ϕ{
	0, 0, -163397, -343421, -44760, 291754, 47776, -211120,
	423168, -254612, -743628, 88586, 197956, -227660, 50990, -45520,
	931917, -841189, 5336, 796608, 41216, -58512, -802672, -72704,
	834560, -949248, 648586, 399990, 0, 0, -589824, -344064,
	711776, -826464, 792896, 255680, 0, 0, -589824, -344064,
	774144, -888832, 760832, 287744, 0, 0, -589824, -344064,
	901292, -973476, 467304, 538768, 0, 0, -634944, -298944,
	933888, -584384, -415940, 631488, 229688, -295870, -598342, 99472,
	933888, -35998, -638140, -358670, -15000, 213120, 99144, -194808,
	930352, 0, 0, -46560, -33280, 79840, -695192, -116840,
	812032, -505980, -44124, 511906, 38198, -255520, -762384, 517996,
	499908, -179520, -715520, -130280, 74936, -95408, 374560, 648496,
	22736, 0, -4368, -155568, -133298, 160352, 132882, 0,
}

// robotoG100 is the to-be-accumulated 'g' from Roboto-Regular.ttf at 100 ppem.
var robotoG100 = []int2ϕ{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -170753, -245717, -106837, -106801, -106801, -85714, 68960, 98079, 98079, 98079, 191498, 252241, 15640, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -147456, -356352, -357437, -187392, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 79292, 317168, 332915, 293150, 26124, 0, 0, 0, 0, 0, -843480, -28968, 0, 0, 0, 0, 0, 0, 596400,
	276048, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -80640, -586656, -380192, -1088, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 153600, 609280, 285696, 0, 0, 0, -35840, -1012736, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, 0, 0, 0, -368640, -592896, -87040, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 353280, 585728, 109568, 0, -113664, -934912, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, 0, -19440, -675904, -353232, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 78696, 775441, 194439, -191488, -857088, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, -13312, -680960, -354304, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 80896, 777216, -78848, -779264, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, -8192, -656384, -384000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 82944, 432128, -515072, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, -591951, -456625, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19564, 94024, -55480, -58108, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 644, -644, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, -250880, -792576, -5120, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 86726, 347746, 352728, 164552, 88452, 8372, 0, -69290, -127488, -144328, -369334, -323106, -15092, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, -26624, -854016, -167936, 0, 0, 0, 0, 0, 0, 0, 0, 0, 49392, 563994, 417582, 17608, 0, 0, 0, 0, 0, 0, 0, 0, 0, -118885, -425685, -452092, -51914, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, -519168, -529408, 0, 0, 0, 0, 0, 0, 0, 0, 0, 245212, 666744, 136620, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -184320, -731136, -133120, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, -124442, -894246, -29888, 0, 0, 0, 0, 0, 0, 0, 0, 226304, 777216, 45056, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -86016, -711680, -250880, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, -529408, -519168, 0, 0, 0, 0, 0, 0, 0, 0, 163824, 804112, 80640, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -27731, -817677, -203168, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, -9216, -916480, -122880, 0, 0, 0, 0, 0, 0, 0, 7168, 835584, 205824, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -168960, -841728, -37888, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, -293888, -754688, 0, 0, 0, 0, 0, 0, 0, 0, 393216, 655360, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -441344, -607232, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, -700416, -348160, 0, 0, 0, 0, 0, 0, 0, 42976, 896976, 108624, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -3052, -864728, -180796, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -15504, -991952, -41120, 0, 0, 0, 0, 0, 0, 0, 374784, 673792, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -189440, -859136, 0, 0, 0, 0, 0, 0, 0, 0, 722944, 325632, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -403456, -645120, 0, 0, 0, 0, 0, 0, 0, 55296, 960512, 32768, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -617472, -431104, 0, 0, 0, 0, 0, 0, 0, 334976, 713600, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -831488, -217088, 0, 0, 0, 0, 0, 0, 0, 522240, 526336, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -1009074, -39502, 0, 0, 0, 0, 0, 0, 0, 699392, 349184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	301056, -1017856, 0, 0, 0, 0, 0, 0, 0, 0, 876544, 172032, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	237568, -954368, 0, 0, 0, 0, 0, 0, 0, 6780, 1022059, 19737, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	174080, -890880, 0, 0, 0, 0, 0, 0, 0, 52224, 996352, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	110592, -827392, 0, 0, 0, 0, 0, 0, 0, 103424, 945152, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	47104, -763904, 0, 0, 0, 0, 0, 0, 0, 154624, 893952, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	26884, -743684, 0, 0, 0, 0, 0, 0, 0, 205824, 842752, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	87040, -803840, 0, 0, 0, 0, 0, 0, 0, 221070, 827506, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	148480, -865280, 0, 0, 0, 0, 0, 0, 0, 166912, 881664, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	209920, -926720, 0, 0, 0, 0, 0, 0, 0, 110592, 937984, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	271360, -988160, 0, 0, 0, 0, 0, 0, 0, 54272, 994304, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	324608, -1033216, -8192, 0, 0, 0, 0, 0, 0, 5698, 1009560, 33318, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -945236, -103340, 0, 0, 0, 0, 0, 0, 0, 839680, 208896, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -742400, -306176, 0, 0, 0, 0, 0, 0, 0, 648192, 400384, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -535552, -513024, 0, 0, 0, 0, 0, 0, 0, 456704, 591872, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -328704, -719872, 0, 0, 0, 0, 0, 0, 0, 200304, 848272, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -121856, -926720, 0, 0, 0, 0, 0, 0, 0, 0, 883712, 164864, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, -696, -951704, -96176, 0, 0, 0, 0, 0, 0, 0, 517120, 531456, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, -598016, -450560, 0, 0, 0, 0, 0, 0, 0, 133872, 890824, 23880, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -3712, -867776, -177088, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, -202752, -845824, 0, 0, 0, 0, 0, 0, 0, 0, 577536, 471040, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -427008, -621568, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, -856064, -192512, 0, 0, 0, 0, 0, 0, 0, 63488, 898048, 87040, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -134144, -863232, -51200, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, -460800, -587776, 0, 0, 0, 0, 0, 0, 0, 0, 324939, 711472, 12165, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -6978, -775948, -265650, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, -82728, -907066, -58782, 0, 0, 0, 0, 0, 0, 0, 0, 423936, 623616, 1024, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -37888, -664576, -346112, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, -449536, -599040, 0, 0, 0, 0, 0, 0, 0, 0, 0, 455762, 565262, 27552, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -98304, -728064, -222208, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, -12288, -828416, -207872, 0, 0, 0, 0, 0, 0, 0, 0, 0, 181248, 657408, 209920, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -32946, -363052, -529302, -123276, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, -217088, -822272, -9216, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12110, 269710, 365305, 336755, 65441, 0, 0, 0, 0, -37053, -141084, -373579, -413205, -83655, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, -567154, -481422, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6042, 84270, 107459, 96990, -72822, -127562, -90630, -3498, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, -6144, -654336, -388096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1680, 38920, -40600, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, -14336, -695296, -338944, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9216, 612352, -375808, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, 0, -26136, -713848, -308592, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 23552, 668672, 356352, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, 0, 0, -6144, -438272, -557056, -47104, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 135240, 635352, 277984, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 716800,
	331776, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -137216, -642048, -269312, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 57344, 501760, 458752, 30720, 0, 0, -802816, -245760, 0, 0, 0, 0, 0, 0, 0, 719836,
	328740, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -4773, -275304, -370431, -336897, -61171, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3000, 198600, 316104, 438864, 92008, 0, 0, 0, 0, -830566, -218010, 0, 0, 0, 0, 0, 0, 0, 779264,
	269312, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -32823, -308432, -244939, -111128, -110851, -110851, -79963, 72183, 92263, 92263, 93800, 226400, 309000, 112800, 0, 0, 0, 0, 0, 0, 0, -928768, -119808, 0, 0, 0, 0, 0, 0, 0, 861184,
	187392, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -5120, -1020928, -22528, 0, 0, 0, 0, 0, 0, 0, 943104,
	105472, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -84992, -963584, 0, 0, 0, 0, 0, 0, 0, 1024, 1022976,
	24576, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -234526, -814050, 0, 0, 0, 0, 0, 0, 0, 75810, 972766,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -582656, -465920, 0, 0, 0, 0, 0, 0, 0, 320512, 728064,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -10240, -931840, -106496, 0, 0, 0, 0, 0, 0, 0, 596992, 451584,
	0, 0, 0, 0, 0, 0, 0, -208896, -125184, 334080, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -275104, -773472, 0, 0, 0, 0, 0, 0, 0, 0, 873472, 175104,
	0, 0, 0, 0, 0, 0, -123904, -822272, -79872, 708608, 317440, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -28672, -819200, -200704, 0, 0, 0, 0, 0, 0, 0, 104628, 942110, 1838,
	0, 0, 0, 0, 0, -60416, -806912, -181248, 0, 27648, 722944, 297984, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -591872, -456704, 0, 0, 0, 0, 0, 0, 0, 0, 531456, 517120, 0,
	0, 0, 0, 0, -19456, -746496, -282624, 0, 0, 0, 34314, 630258, 383379, 625, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -336442, -709198, -2936, 0, 0, 0, 0, 0, 0, 0, 71680, 916480, 60416, 0,
	0, 0, 0, -1024, -641024, -406528, 0, 0, 0, 0, 0, 0, 313344, 626688, 108544, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -12288, -518144, -507904, -10240, 0, 0, 0, 0, 0, 0, 0, 0, 539648, 508928, 0, 0,
	0, 0, 0, -404636, -643940, 0, 0, 0, 0, 0, 0, 0, 0, 66744, 548784, 379656, 53392, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -146253, -642501, -259822, 0, 0, 0, 0, 0, 0, 0, 0, 0, 94704, 897266, 56606, 0, 0,
	0, 0, 0, -76800, -823296, -148480, 0, 0, 0, 0, 0, 0, 0, 0, 0, 41082, 326652, 379990, 283672, 17412, 0, 0, 0, 0, 0, 0, -71680, -350208, -378611, -244736, -4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 25600, 777216, 245760, 0, 0, 0,
	0, 0, 0, 0, -162816, -819200, -66560, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19272, 105996, 116508, 116508, 57084, -97986, -106188, -106188, -93534, -11502, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 644096, 404480, 0, 0, 0, 0,
	0, 0, 0, 0, 0, -246228, -716428, -85920, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 462866, 585710, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, -124928, -717824, -205824, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12288, 542720, 489472, 4096, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, -37467, -578681, -417998, -14430, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 83968, 686080, 278528, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, -169984, -557056, -320512, -1024, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5360, 323064, 597176, 122976, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -243696, -433584, -331280, -40016, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11264, 351232, 501760, 184320, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -56364, -346968, -358712, -230468, -56064, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 38808, 239085, 267450, 350232, 153646, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -9756, -172898, -228979, -170916, -76130, -75226, -74959, 16896, 79577, 79577, 79577, 84689, 211364, 228879, 28224, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}
