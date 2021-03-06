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
	"math"

	"golang.org/x/image/math/f32"
)

func max(x, y float32) float32 {
	if x > y {
		return x
	}
	return y
}

func min(x, y float32) float32 {
	if x < y {
		return x
	}
	return y
}

func floor(x float32) int32 { return int32(math.Floor(float64(x))) }
func ceil(x float32) int32  { return int32(math.Ceil(float64(x))) }

func clamp(i, width int32) uint {
	if i < 0 {
		return 0
	}
	if i < width {
		return uint(i)
	}
	return uint(width)
}

func concat(a, b *f32.Aff3) f32.Aff3 {
	return f32.Aff3{
		a[0]*b[0] + a[1]*b[3],
		a[0]*b[1] + a[1]*b[4],
		a[0]*b[2] + a[1]*b[5] + a[2],
		a[3]*b[0] + a[4]*b[3],
		a[3]*b[1] + a[4]*b[4],
		a[3]*b[2] + a[4]*b[5] + a[5],
	}
}

type op uint8

const (
	moveTo op = 0
	lineTo op = 1
	quadTo op = 2
)

type point struct {
	x, y float32
}

func midPoint(p, q point) point {
	return point{
		x: (p.x + q.x) * 0.5,
		y: (p.y + q.y) * 0.5,
	}
}

func lerp(t float32, p, q point) point {
	return point{
		x: p.x + t*(q.x-p.x),
		y: p.y + t*(q.y-p.y),
	}
}

func mul(m *f32.Aff3, p point) point {
	return point{
		x: m[0]*p.x + m[1]*p.y + m[2],
		y: m[3]*p.x + m[4]*p.y + m[5],
	}
}

type segment struct {
	op   op
	p, q point
}

type rasterizer struct {
	a     []float32
	first point
	last  point
	w     int
	h     int
}

func newRasterizer(w, h int) *rasterizer {
	return &rasterizer{
		a: make([]float32, w*h),
		w: w,
		h: h,
	}
}

func (z *rasterizer) Bounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{z.w, z.h}}
}

func (z *rasterizer) reset() {
	for i := range z.a {
		z.a[i] = 0
	}
	z.first = point{}
	z.last = point{}
}

func (z *rasterizer) rasterize(f *Font, a glyphData, transform f32.Aff3) {
	g := a.glyphIter()
	if g.compoundGlyph() {
		for g.nextSubGlyph() {
			z.rasterize(f, f.glyphData(g.subGlyphID), concat(&transform, &g.subTransform))
		}
		return
	}

	for g.nextContour() {
		for g.nextSegment() {
			switch g.seg.op {
			case moveTo:
				p := mul(&transform, g.seg.p)
				z.moveTo(p)
			case lineTo:
				p := mul(&transform, g.seg.p)
				z.lineTo(p)
			case quadTo:
				p := mul(&transform, g.seg.p)
				q := mul(&transform, g.seg.q)
				z.quadTo(p, q)
			}
		}
	}
}

func accumulate(dst []uint8, src []float32) {
	// almost256 scales a floating point value in the range [0, 1] to a uint8
	// value in the range [0x00, 0xff].
	//
	// 255 is too small. Floating point math accumulates rounding errors, so a
	// fully covered src value that would in ideal math be float32(1) might be
	// float32(1-ε), and uint8(255 * (1-ε)) would be 0xfe instead of 0xff. The
	// uint8 conversion rounds to zero, not to nearest.
	//
	// 256 is too big. If we multiplied by 256, below, then a fully covered src
	// value of float32(1) would translate to uint8(256 * 1), which can be 0x00
	// instead of the maximal value 0xff.
	//
	// math.Float32bits(almost256) is 0x437fffff.
	const almost256 = 255.99998

	// TODO: pix adjustment if dst.Bounds() != z.Bounds()?
	acc := float32(0)
	for i, v := range src {
		acc += v
		a := acc
		if a < 0 {
			a = -a
		}
		if a > 1 {
			a = 1
		}
		dst[i] = uint8(almost256 * a)
	}
}

func (z *rasterizer) closePath() {
	z.lineTo(z.first)
}

func (z *rasterizer) moveTo(p point) {
	z.first = p
	z.last = p
}

func (z *rasterizer) lineTo(q point) {
	p := z.last
	z.last = q
	dir := float32(1)
	if p.y > q.y {
		dir, p, q = -1, q, p
	}
	// Horizontal line segments yield no change in coverage. Almost horizontal
	// segments would yield some change, in ideal math, but the computation
	// further below, involving 1 / (q.y - p.y), is unstable in floating point
	// math, so we treat the segment as if it was perfectly horizontal.
	if q.y-p.y <= 0.000001 {
		return
	}
	dxdy := (q.x - p.x) / (q.y - p.y)

	x := p.x
	y := floor(p.y)
	yMax := ceil(q.y)
	if yMax > int32(z.h) {
		yMax = int32(z.h)
	}
	width := int32(z.w)

	for ; y < yMax; y++ {
		dy := min(float32(y+1), q.y) - max(float32(y), p.y)
		xNext := x + dy*dxdy
		if y < 0 {
			x = xNext
			continue
		}
		buf := z.a[y*width:]
		d := dy * dir
		x0, x1 := x, xNext
		if x > xNext {
			x0, x1 = x1, x0
		}
		x0i := floor(x0)
		x0Floor := float32(x0i)
		x1i := ceil(x1)
		x1Ceil := float32(x1i)

		if x1i <= x0i+1 {
			xmf := 0.5*(x+xNext) - x0Floor
			if i := clamp(x0i+0, width); i < uint(len(buf)) {
				buf[i] += d - d*xmf
			}
			if i := clamp(x0i+1, width); i < uint(len(buf)) {
				buf[i] += d * xmf
			}
		} else {
			s := 1 / (x1 - x0)
			x0f := x0 - x0Floor
			oneMinusX0f := 1 - x0f
			a0 := 0.5 * s * oneMinusX0f * oneMinusX0f
			x1f := x1 - x1Ceil + 1
			am := 0.5 * s * x1f * x1f

			if i := clamp(x0i, width); i < uint(len(buf)) {
				buf[i] += d * a0
			}

			if x1i == x0i+2 {
				if i := clamp(x0i+1, width); i < uint(len(buf)) {
					buf[i] += d * (1 - a0 - am)
				}
			} else {
				a1 := s * (1.5 - x0f)
				if i := clamp(x0i+1, width); i < uint(len(buf)) {
					buf[i] += d * (a1 - a0)
				}
				dTimesS := d * s
				for xi := x0i + 2; xi < x1i-1; xi++ {
					if i := clamp(xi, width); i < uint(len(buf)) {
						buf[i] += dTimesS
					}
				}
				a2 := a1 + s*float32(x1i-x0i-3)
				if i := clamp(x1i-1, width); i < uint(len(buf)) {
					buf[i] += d * (1 - a2 - am)
				}
			}

			if i := clamp(x1i, width); i < uint(len(buf)) {
				buf[i] += d * am
			}
		}

		x = xNext
	}
}

func (z *rasterizer) quadTo(q, r point) {
	// We make a linear approximation to the curve.
	// http://lists.nongnu.org/archive/html/freetype-devel/2016-08/msg00080.html
	// gives the rationale for this evenly spaced heuristic instead of a
	// recursive de Casteljau approach:
	//
	// The reason for the subdivision by n is that I expect the "flatness"
	// computation to be semi-expensive (it's done once rather than on each
	// potential subdivision) and also because you'll often get fewer
	// subdivisions. Taking a circular arc as a simplifying assumption (ie a
	// spherical 🐄), where I get n, a recursive approach would get 2^⌈lg n⌉,
	// which, if I haven't made any horrible mistakes, is expected to be 33%
	// more in the limit.
	p := z.last
	devx := p.x - 2*q.x + r.x
	devy := p.y - 2*q.y + r.y
	devsq := devx*devx + devy*devy
	if devsq >= 0.333 {
		const tol = 3
		n := 1 + int(math.Sqrt(math.Sqrt(tol*float64(devsq))))
		t, nInv := float32(0), 1/float32(n)
		for i := 0; i < n-1; i++ {
			t += nInv
			s := lerp(t, lerp(t, p, q), lerp(t, q, r))
			z.lineTo(s)
		}
	}
	z.lineTo(r)
}
