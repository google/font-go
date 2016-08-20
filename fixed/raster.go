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
	"golang.org/x/image/math/fixed"
)

func max(x, y fixed.Int26_6) fixed.Int26_6 {
	if x > y {
		return x
	}
	return y
}

func min(x, y fixed.Int26_6) fixed.Int26_6 {
	if x < y {
		return x
	}
	return y
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

// int20_12 is a signed 20.12 fixed-point number.
type int20_12 int32

type rasterizer struct {
	a    []int20_12
	last point
	w    int
	h    int
}

func newRasterizer(w, h int) *rasterizer {
	return &rasterizer{
		a: make([]int20_12, w*h),
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
				z.last = mul(&transform, g.seg.p)
			case lineTo:
				p := mul(&transform, g.seg.p)
				z.drawLine(z.last, p)
				z.last = p
			case quadTo:
				p := mul(&transform, g.seg.p)
				q := mul(&transform, g.seg.q)
				z.drawQuad(z.last, p, q)
				z.last = q
			}
		}
	}
}

func accumulate(dst []uint8, src []int20_12) {
	// TODO: pix adjustment if dst.Bounds() != z.Bounds()?
	acc := int20_12(0)
	for i, v := range src {
		acc += v
		a := acc
		if a < 0 {
			a = -a
		}
		if a > 0xfff {
			a = 0xfff
		}
		dst[i] = uint8(a >> 4)
	}
}

const debugOutOfBounds = false

func (z *rasterizer) drawLine(p, q point) {
	px := fixed.Int26_6(p.x * (1 << 6))
	py := fixed.Int26_6(p.y * (1 << 6))
	qx := fixed.Int26_6(q.x * (1 << 6))
	qy := fixed.Int26_6(q.y * (1 << 6))
	if py == qy {
		return
	}
	dir := fixed.Int26_6(1)
	if py > qy {
		dir, px, py, qx, qy = -1, qx, qy, px, py
	}
	deltax, deltay := qx - px, qy - py

	x := px
	if py < 0 {
		x -= py * deltax / deltay
	}
	// TODO: floor instead of round to zero? Make this max(0, etc)? int instead of uint is more Go-like.
	y := int32(py+0x00) >> 6
	yMax := int32(qy+0x3f) >> 6
	if yMax > int32(z.h) {
		yMax = int32(z.h)
	}

	for ; y < yMax; y++ {
		buf := z.a[y*int32(z.w):]
		dy := min(fixed.Int26_6(y+1)<<6, qy) - max(fixed.Int26_6(y)<<6, py)
		xNext := x + dy*deltax/deltay
		d := dy * dir
		x0, x1 := x, xNext
		if x > xNext {
			x0, x1 = x1, x0
		}
		x0i := int32(x0+0x00) >> 6
		x0Floor := fixed.Int26_6(x0i) << 6
		x1i := int32(x1+0x3f) >> 6
		x1Ceil := fixed.Int26_6(x1i) << 6

		if x1i <= x0i+1 {
			xmf := (x+xNext)>>1 - x0Floor
			if i := uint(x0i + 0); i < uint(len(buf)) {
				buf[i] += int20_12(d * (1<<6 - xmf))
			} else if debugOutOfBounds {
				println("out of bounds #0")
			}
			if i := uint(x0i + 1); i < uint(len(buf)) {
				buf[i] += int20_12(d * xmf)
			} else if debugOutOfBounds {
				println("out of bounds #1")
			}
		} else {
			oneOverS := x1 - x0
			x0f := x0 - x0Floor
			oneMinusX0f := 1<<6 - x0f
			a0 := ((oneMinusX0f * oneMinusX0f) >> 1) / oneOverS
			x1f := x1 - x1Ceil + 1<<6
			am := ((x1f * x1f) >> 1) / oneOverS

			if i := uint(x0i); i < uint(len(buf)) {
				buf[i] += int20_12(d * a0)
			} else if debugOutOfBounds {
				println("out of bounds #2")
			}

			if x1i == x0i+2 {
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += int20_12(d * (1<<6 - a0 - am))
				} else if debugOutOfBounds {
					println("out of bounds #3")
				}
			} else {
				a1 := ((1<<6 + 1<<5 - x0f) << 6) / oneOverS
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += int20_12(d * (a1 - a0))
				} else if debugOutOfBounds {
					println("out of bounds #4")
				}
				dTimesS := int20_12((d << 12) / oneOverS)
				for xi := x0i + 2; xi < x1i-1; xi++ {
					if i := uint(xi); i < uint(len(buf)) {
						buf[i] += dTimesS
					} else if debugOutOfBounds {
						println("out of bounds #5")
					}
				}
				a2 := a1 + (fixed.Int26_6(x1i-x0i-3)<<12)/oneOverS
				if i := uint(x1i - 1); i < uint(len(buf)) {
					buf[i] += int20_12(d * (1<<6 - a2 - am))
				} else if debugOutOfBounds {
					println("out of bounds #6")
				}
			}

			if i := uint(x1i); i < uint(len(buf)) {
				buf[i] += int20_12(d * am)
			} else if debugOutOfBounds {
				println("out of bounds #7")
			}
		}

		x = xNext
	}
}

func (z *rasterizer) drawQuad(p, q, r point) {
	devx := p.x - 2*q.x + r.x
	devy := p.y - 2*q.y + r.y
	devsq := devx*devx + devy*devy
	if devsq < 0.333 {
		z.drawLine(p, r)
		return
	}
	const tol = 3
	n := 1 + int(math.Floor(math.Sqrt(math.Sqrt(tol*float64(devsq)))))
	t, nInv := float32(0), 1/float32(n)
	last := p
	for i := 0; i < n-1; i++ {
		t += nInv
		s := lerp(t, lerp(t, p, q), lerp(t, q, r))
		z.drawLine(last, s)
		last = s
	}
	z.drawLine(last, r)
}
