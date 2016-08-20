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
	"testing"
)

func TestAccumulateSIMDUnaligned(t *testing.T) {
	if !haveAccumulateSIMD {
		t.Skip("No accumulateSIMD implemention")
	}

	dst := make([]uint8, 64)
	src := make([]float32, 64)

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

	dst := make([]uint8, 4)
	src := []float32{0.25, 0.25, 0.25, 0.25}
	accumulateSIMD(dst[:0], src)
	for i, got := range dst {
		if got != 0 {
			t.Errorf("i=%d: got %#02x, want %#02x", i, got, 0)
		}
	}
}

func TestAccumulate(t *testing.T)            { testAccumulate(t, sequence, sequenceAcc, false) }
func TestAccumulateSIMD(t *testing.T)        { testAccumulate(t, sequence, sequenceAcc, true) }
func TestAccumulateRobotoG(t *testing.T)     { testAccumulate(t, robotoG16, robotoG16Acc, false) }
func TestAccumulateSIMDRobotoG(t *testing.T) { testAccumulate(t, robotoG16, robotoG16Acc, true) }

func BenchmarkAccumulate16(b *testing.B)      { benchAccumulate(b, robotoG16, false) }
func BenchmarkAccumulateSIMD16(b *testing.B)  { benchAccumulate(b, robotoG16, true) }
func BenchmarkAccumulate100(b *testing.B)     { benchAccumulate(b, robotoG100, false) }
func BenchmarkAccumulateSIMD100(b *testing.B) { benchAccumulate(b, robotoG100, true) }

func testAccumulate(t *testing.T, src []float32, want []byte, simd bool) {
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

func benchAccumulate(b *testing.B, src []float32, simd bool) {
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
	0x1f,
	0x5f,
	0x1f,
	0x3f,
	0x5f,
	0x5f,
	0x9f,
	0xff,
	0xdf,
	0x00,
	0x3f,
}

var sequence = []float32{
	+0.125, // Running sum: +0.125
	-0.500, // Running sum: -0.375
	+0.250, // Running sum: -0.125
	+0.375, // Running sum: +0.250
	+0.125, // Running sum: +0.375
	+0.000, // Running sum: +0.375
	-1.000, // Running sum: -0.625
	-0.500, // Running sum: -1.125
	+0.250, // Running sum: -0.875
	+0.875, // Running sum: +0.000
	+0.250, // Running sum: +0.250
}

// robotoG16Acc is the accumulation of roboto16.
var robotoG16Acc = []uint8{
	0x00, 0x00, 0x27, 0x7b, 0x86, 0x3f, 0x33, 0x66,
	0x00, 0x3e, 0xf2, 0xdd, 0xad, 0xe4, 0xd8, 0xe3,
	0x00, 0xcc, 0xcb, 0x0a, 0x00, 0x0e, 0xd1, 0xe3,
	0x18, 0xfe, 0x61, 0x00, 0x00, 0x00, 0x8f, 0xe3,
	0x36, 0xfe, 0x3e, 0x00, 0x00, 0x00, 0x8f, 0xe3,
	0x26, 0xfe, 0x46, 0x00, 0x00, 0x00, 0x8f, 0xe3,
	0x07, 0xf4, 0x83, 0x00, 0x00, 0x00, 0x9a, 0xe3,
	0x00, 0x8d, 0xf3, 0x59, 0x21, 0x69, 0xfb, 0xe3,
	0x00, 0x08, 0xa3, 0xfb, 0xfe, 0xca, 0xb2, 0xe2,
	0x00, 0x00, 0x00, 0x0b, 0x13, 0x00, 0xa9, 0xc5,
	0x00, 0x7a, 0x85, 0x09, 0x00, 0x3e, 0xf7, 0x79,
	0x00, 0x2b, 0xd9, 0xf9, 0xe7, 0xfe, 0xa3, 0x05,
	0x00, 0x00, 0x01, 0x26, 0x47, 0x20, 0x00, 0x00,
}

// robotoG16 is the to-be-accumulated 'g' from Roboto-Regular.ttf at 16 ppem.
var robotoG16 = []float32{
	0, 0, -0.15590487, -0.3276114, -0.042107075, 0.2775064, 0.045715243, -0.20116276,
	0.40356445, -0.24355067, -0.70834696, 0.084576145, 0.18841398, -0.21646743, 0.047490746, -0.042740747,
	0.8883049, -0.8013918, 0.0051045865, 0.7592861, 0.03932122, -0.055804897, -0.7642893, -0.070530795,
	0.7955679, -0.9049429, 0.6187296, 0.3812704, 0, 0, -0.5625, -0.328125,
	0.67793864, -0.78731364, 0.7563377, 0.2436623, 0, 0, -0.5625, -0.328125,
	0.73970735, -0.84908235, 0.72424936, 0.27575064, 0, 0, -0.5625, -0.328125,
	0.86021227, -0.9287864, 0.44457173, 0.51462746, 0, 0, -0.60559565, -0.28502935,
	0.890625, -0.5562324, -0.39759094, 0.60200083, 0.21881261, -0.2821433, -0.57003665, 0.0945648,
	0.890625, -0.034569852, -0.6071909, -0.34315312, -0.0143135525, 0.20355242, 0.0937576, -0.18528756,
	0.88720495, 0, 0, -0.04441565, -0.031537995, 0.07595364, -0.6635582, -0.11050351,
	0.7740617, -0.4820481, -0.042926386, 0.48842722, 0.036547318, -0.24379939, -0.7267487, 0.4940294,
	0.4765187, -0.17161918, -0.6807058, -0.12546086, 0.07122832, -0.09014779, 0.35693973, 0.61824495,
	0.021520678, 0, -0.0042853137, -0.14814371, -0.12679476, 0.1524287, 0.12679507, 0,
}

// robotoG100 is the to-be-accumulated 'g' from Roboto-Regular.ttf at 100 ppem.
var robotoG100 = []float32{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.16257538, -0.23449227, -0.1022292, -0.10183924, -0.10183924, -0.08090962, 0.06534107, 0.09352037, 0.09352037, 0.09352037, 0.18309283, 0.23994578, 0.014944109, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.14141704, -0.33945537, -0.34080547, -0.17823012, -9.201216e-05, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.076266244, 0.30251613, 0.3174602, 0.2781146, 0.025642795, 0, 0, 0, 0, 0, -0.80436397, -0.027667299, 0, 0, 0, 0, 0, 0, 0.56877136,
	0.2632599, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.07731551, -0.55889654, -0.3621843, -0.001603625, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.1464724, 0.57977307, 0.27375454, 0, 0, 0, -0.03547615, -0.9644958, -2.8079477e-05, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, 0, 0, -0.0007365446, -0.35111147, -0.5648665, -0.0832855, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.0008237954, 0.3354711, 0.5578763, 0.10582883, 0, -0.110450745, -0.88954926, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, 0, -0.018768838, -0.6433475, -0.33788368, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.07566233, 0.7385571, 0.18578053, -0.1854477, -0.8145523, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, -0.0128355585, -0.64837134, -0.3387931, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.07722519, 0.740098, -0.07777549, -0.7395477, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, -0.008026161, -0.625023, -0.3669508, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.07924781, 0.40570384, -0.48495167, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, -0.5645168, -0.43548325, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.018709186, 0.08947051, -0.052624084, -0.055555608, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.00039970875, -0.00039970875, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, -0.24043435, -0.75452244, -0.005043202, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.082909115, 0.33201638, 0.33522737, 0.1576198, 0.08413116, 0.008096158, -0.00011755934, -0.066013046, -0.12156858, -0.13745096, -0.3528051, -0.3072366, -0.014808126, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, -0.025920385, -0.8137927, -0.1602869, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.047159743, 0.5371427, 0.39887682, 0.016820818, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.113577805, -0.40600643, -0.43049487, -0.049920887, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, -0.49587584, -0.50412416, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.23389316, 0.6357093, 0.13039753, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.17586108, -0.6964623, -0.12767668, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, -0.1187886, -0.8528714, -0.028339991, 0, 0, 0, 0, 0, 0, 0, 0, 0.21607055, 0.74034476, 0.04358471, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.08205361, -0.6776248, -0.24032158, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, -0.50402546, -0.49597454, 0, 0, 0, 0, 0, 0, 0, 0, 0.15690169, 0.7661239, 0.07697441, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.02653598, -0.77872443, -0.19473957, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -0.009634551, -0.8730738, -0.11729162, 0, 0, 0, 0, 0, 0, 0, 0.0077682864, 0.7955067, 0.19672503, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.16065413, -0.8021239, -0.03722197, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -0.28066015, -0.71933985, 0, 0, 0, 0, 0, 0, 0, 0, 0.3764, 0.6236, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.41836166, -0.58163834, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -0.66897774, -0.33102226, 0, 0, 0, 0, 0, 0, 0, 0.042135485, 0.8550727, 0.10279185, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.003072831, -0.8233886, -0.17353858, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.01483847, -0.94559807, -0.03956348, 0, 0, 0, 0, 0, 0, 0, 0.3571434, 0.6428566, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.18039083, -0.81960917, 0, 0, 0, 0, 0, 0, 0, 0, 0.6896019, 0.3103981, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.3852346, -0.6147654, 0, 0, 0, 0, 0, 0, 0, 0.053319424, 0.9154215, 0.03125903, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.5900786, -0.4099214, 0, 0, 0, 0, 0, 0, 0, 0.31948417, 0.6805158, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.79492235, -0.20507765, 0, 0, 0, 0, 0, 0, 0, 0.49865532, 0.5013447, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.962837, -0.037163034, 0, 0, 0, 0, 0, 0, 0, 0.6681709, 0.33182907, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.28685185, -0.9704388, -6.8452573e-06, 0, 0, 0, 0, 0, 0, 0, 0.83768845, 0.16231155, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.22593713, -0.9095309, 0, 0, 0, 0, 0, 0, 0, 0.0066761267, 0.975568, 0.0177559, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.16501546, -0.8486092, 0, 0, 0, 0, 0, 0, 0, 0.050175667, 0.94982433, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.10409391, -0.78768766, 0, 0, 0, 0, 0, 0, 0, 0.099303246, 0.90069675, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.04317224, -0.726766, 0, 0, 0, 0, 0, 0, 0, 0.14843082, 0.8515692, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.0260292, -0.709623, 0, 0, 0, 0, 0, 0, 0, 0.1975584, 0.8024416, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.083500266, -0.767094, 0, 0, 0, 0, 0, 0, 0, 0.21111052, 0.78888947, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.14222085, -0.8258146, 0, 0, 0, 0, 0, 0, 0, 0.15846825, 0.84153175, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.20094156, -0.8845353, 0, 0, 0, 0, 0, 0, 0, 0.104335785, 0.8956642, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.25966215, -0.9432559, 0, 0, 0, 0, 0, 0, 0, 0.050203323, 0.9497967, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.3100212, -0.9852533, -0.008361637, 0, 0, 0, 0, 0, 0, 0.0049284007, 0.9628471, 0.032224406, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.9002044, -0.09979555, 0, 0, 0, 0, 0, 0, 0, 0.7997799, 0.20022011, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.70645475, -0.29354525, 0, 0, 0, 0, 0, 0, 0, 0.61662865, 0.38337135, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.50830984, -0.49169016, 0, 0, 0, 0, 0, 0, 0, 0.4334793, 0.5665207, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.3101647, -0.6898353, 0, 0, 0, 0, 0, 0, 0, 0.18966764, 0.81033236, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.11201978, -0.8879802, 0, 0, 0, 0, 0, 0, 0, 0.00038834772, 0.8407222, 0.15888949, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, -0.00042299856, -0.90386456, -0.09571239, 0, 0, 0, 0, 0, 0, 0, 0.49152184, 0.50847816, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -0.56883097, -0.43116903, 0, 0, 0, 0, 0, 0, 0, 0.12692434, 0.84963375, 0.02344187, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.003698804, -0.8273135, -0.16898772, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -0.19150305, -0.80849695, 0, 0, 0, 0, 0, 0, 0, 0, 0.5488987, 0.4511013, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.40703583, -0.5929642, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, -1.0679125e-05, -0.81415343, -0.1858359, 0, 0, 0, 0, 0, 0, 0, 0.059720416, 0.8553741, 0.08490552, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.12886734, -0.8222538, -0.04887886, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, -0.43684673, -0.56315327, 0, 0, 0, 0, 0, 0, 0, 0, 0.3096562, 0.67849547, 0.011848275, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.0067449138, -0.7402579, -0.2529972, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, -0.07788561, -0.8651974, -0.056917027, 0, 0, 0, 0, 0, 0, 0, 0, 0.40395403, 0.5947437, 0.00130226, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.036737923, -0.6324377, -0.33082438, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, -0.42711973, -0.57288027, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.4344076, 0.5391021, 0.026490372, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.09393775, -0.69413877, -0.21192348, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, -0.01150534, -0.7880039, -0.20049068, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.17303762, 0.62628245, 0.20067988, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.031736482, -0.34536293, -0.5054345, -0.117466055, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, -0.2052985, -0.7843126, -0.010388919, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.01224111, 0.25738782, 0.34833333, 0.31977242, 0.062265366, 0, 0, 0, 0, -0.035292372, -0.13432634, -0.35645485, -0.3937764, -0.08015011, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, -0.53860474, -0.46139526, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.0061193565, 0.08051613, 0.10245097, 0.091371186, -0.06899115, -0.12165264, -0.08636027, -0.003453594, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, -0.006174167, -0.62300813, -0.37081775, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.0017395883, 0.036950663, -0.03869024, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, -0.013994738, -0.6619886, -0.32401663, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.00942973, 0.583218, -0.35827276, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, 0, -0.02497123, -0.68019724, -0.2948315, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.023257846, 0.6370361, 0.33970606, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, 0, 0, -0.0068311584, -0.4186462, -0.52945596, -0.045066655, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.12906657, 0.60652125, 0.2644122, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68359375,
	0.31640625, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.13145107, -0.6112677, -0.25728124, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.05542326, 0.47796154, 0.43680432, 0.029810887, 0, 0, -0.765625, -0.234375, 0, 0, 0, 0, 0, 0, 0, 0.68643564,
	0.31356436, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.0045727356, -0.26356637, -0.35274172, -0.32069632, -0.058422845, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.0032209016, 0.1890101, 0.30106902, 0.41879046, 0.08790954, 0, 0, 0, 0, -0.7920615, -0.20793848, 0, 0, 0, 0, 0, 0, 0, 0.74381256,
	0.25618744, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.03170294, -0.29397637, -0.23442516, -0.105759375, -0.10569949, -0.10569949, -0.07527236, 0.06927164, 0.08797251, 0.08797251, 0.089483395, 0.2160986, 0.29376286, 0.10797365, 0, 0, 0, 0, 0, 0, 0, -0.8870964, -0.112903595, 0, 0, 0, 0, 0, 0, 0, 0.82205963,
	0.17794037, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.0061938707, -0.9732728, -0.020533318, 0, 0, 0, 0, 0, 0, 0, 0.90031433,
	0.09968567, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.0842247, -0.9157753, 0, 0, 0, 0, 0, 0, 0, 0.001999287, 0.9745648,
	0.023435978, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.22505447, -0.7749455, 0, 0, 0, 0, 0, 0, 0, 0.072779834, 0.92722017,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.55506897, -0.44493103, 0, 0, 0, 0, 0, 0, 0, 0.3051834, 0.6948166,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.01006535, -0.8877424, -0.1021922, 0, 0, 0, 0, 0, 0, 0, 0.56951904, 0.43048096,
	0, 0, 0, 0, 0, 0, 0, -0.19995248, -0.1180287, 0.31798118, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.2623227, -0.7376773, 0, 0, 0, 0, 0, 0, 0, 0, 0.83384705, 0.16615295,
	0, 0, 0, 0, 0, 0, -0.11861901, -0.7832962, -0.076160155, 0.6747583, 0.30331713, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.027721029, -0.78118396, -0.19109498, 0, 0, 0, 0, 0, 0, 0, 0.10161246, 0.89620274, 0.0021847554,
	0, 0, 0, 0, 0, -0.058400378, -0.7685983, -0.17300127, 0, 0.027145138, 0.68764114, 0.28521368, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.56539154, -0.43460846, 0, 0, 0, 0, 0, 0, 0, 0, 0.5068283, 0.4931717, 0,
	0, 0, 0, 0, -0.01929663, -0.71167064, -0.2690327, 0, 0, 0, 0.032922663, 0.60077965, 0.3654961, 0.00080157793, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.32239795, -0.6742182, -0.0033838397, 0, 0, 0, 0, 0, 0, 0, 0.0685621, 0.8737534, 0.057684492, 0,
	0, 0, 0, -0.0013077562, -0.61251324, -0.38617897, 0, 0, 0, 0, 0, 0, 0.29901797, 0.5969552, 0.104026854, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.012386264, -0.49387312, -0.4835369, -0.010203709, 0, 0, 0, 0, 0, 0, 0, 0, 0.5149231, 0.4850769, 0, 0,
	0, 0, 0, -0.38716474, -0.6128353, 0, 0, 0, 0, 0, 0, 0, 0, 0.06380953, 0.5227931, 0.3624985, 0.050898876, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.1395998, -0.6128111, -0.24758911, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.09063255, 0.8554909, 0.05387651, 0, 0,
	0, 0, 0, -0.07297163, -0.7844292, -0.14259917, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.040019367, 0.31146762, 0.36236653, 0.26958504, 0.016561385, 0, 0, 0, 0, 0, 0, -0.06851367, -0.33443567, -0.36104682, -0.23174116, -0.004262717, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.024808059, 0.74039054, 0.23480143, 0, 0, 0,
	0, 0, 0, 0, -0.15545118, -0.7802954, -0.06425346, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.018693922, 0.10131476, 0.111111104, 0.111111104, 0.054450817, -0.09355072, -0.10126497, -0.10126497, -0.08911653, -0.011484552, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.00043438628, 0.6136427, 0.3859229, 0, 0, 0, 0,
	0, 0, 0, 0, 0, -0.23450266, -0.6831598, -0.082337566, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.44162422, 0.5583758, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, -0.11969634, -0.6831485, -0.1971552, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.012228634, 0.51775604, 0.46608824, 0.0039270995, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, -0.035681423, -0.5521471, -0.3982963, -0.013875188, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.08056112, 0.6540233, 0.26541564, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, -0.16317414, -0.53055257, -0.3052079, -0.0010653809, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.0050811404, 0.30709738, 0.5709021, 0.116919376, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.23248357, -0.41394642, -0.31492975, -0.038640246, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.011194665, 0.33431658, 0.47795287, 0.17653587, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.054347016, -0.33063647, -0.34170943, -0.22017153, -0.05313553, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.036833737, 0.2277294, 0.2550052, 0.3332095, 0.14722216, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -0.009979362, -0.1652037, -0.21833928, -0.1619393, -0.07284279, -0.0717369, -0.071060844, 0.015526393, 0.075885855, 0.075885855, 0.075885855, 0.0806102, 0.2018606, 0.21817161, 0.027275827, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}
