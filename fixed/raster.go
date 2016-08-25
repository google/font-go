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

const (
	// ϕ is the number of binary digits after the fixed point.
	//
	// For example, if ϕ == 10 (and int1ϕ is based on the int32 type) then we
	// are using 22.10 fixed point math.
	ϕ = 6

	one          = 1 << ϕ
	oneAndAHalf  = 1<<ϕ + 1<<(ϕ-1)
	oneMinusIota = 1<<ϕ - 1 // Used for rounding up.
)

// int2ϕ is a signed fixed-point number with 2*ϕ binary digits after the fixed
// point.
type int2ϕ int32

// int1ϕ is a signed fixed-point number with 1*ϕ binary digits after the fixed
// point.
type int1ϕ int32

func max(x, y int1ϕ) int1ϕ {
	if x > y {
		return x
	}
	return y
}

func min(x, y int1ϕ) int1ϕ {
	if x < y {
		return x
	}
	return y
}

func floor(x int1ϕ) int32 { return int32(x >> ϕ) }
func ceil(x int1ϕ) int32  { return int32((x + oneMinusIota) >> ϕ) }

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
	a    []int2ϕ
	last point
	w    int
	h    int
}

func newRasterizer(w, h int) *rasterizer {
	return &rasterizer{
		a: make([]int2ϕ, w*h),
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

func accumulate(dst []uint8, src []int2ϕ) {
	// TODO: pix adjustment if dst.Bounds() != z.Bounds()?
	acc := int2ϕ(0)
	for i, v := range src {
		acc += v
		a := acc
		if a < 0 {
			a = -a
		}
		a >>= 2*ϕ - 8
		if a > 0xff {
			a = 0xff
		}
		dst[i] = uint8(a)
	}
}

const debugOutOfBounds = false

func (z *rasterizer) drawLine(p, q point) {
	if p.y == q.y {
		return
	}
	dir := int1ϕ(1)
	if p.y > q.y {
		dir, p, q = -1, q, p
	}
	dxdy := (q.x - p.x) / (q.y - p.y)
	py := int1ϕ(p.y * one)
	qy := int1ϕ(q.y * one)

	x := int1ϕ(p.x * one)
	if p.y < 0 {
		x -= int1ϕ(float32(one) * p.y * dxdy)
	}
	y := floor(py)
	yMax := ceil(qy)
	if yMax > int32(z.h) {
		yMax = int32(z.h)
	}

	for ; y < yMax; y++ {
		buf := z.a[y*int32(z.w):]
		dy := min(int1ϕ(y+1)<<ϕ, qy) - max(int1ϕ(y)<<ϕ, py)
		xNext := x + int1ϕ(float32(dy)*dxdy)
		d := dy * dir
		x0, x1 := x, xNext
		if x > xNext {
			x0, x1 = x1, x0
		}
		x0i := floor(x0)
		x0Floor := int1ϕ(x0i) << ϕ
		x1i := ceil(x1)
		x1Ceil := int1ϕ(x1i) << ϕ

		if x1i <= x0i+1 {
			xmf := (x+xNext)>>1 - x0Floor
			if i := uint(x0i + 0); i < uint(len(buf)) {
				buf[i] += int2ϕ(d * (one - xmf))
			} else if debugOutOfBounds {
				println("out of bounds #0")
			}
			if i := uint(x0i + 1); i < uint(len(buf)) {
				buf[i] += int2ϕ(d * xmf)
			} else if debugOutOfBounds {
				println("out of bounds #1")
			}
		} else {
			oneOverS := x1 - x0
			x0f := x0 - x0Floor
			oneMinusX0f := one - x0f
			a0 := ((oneMinusX0f * oneMinusX0f) >> 1) / oneOverS
			x1f := x1 - x1Ceil + one
			am := ((x1f * x1f) >> 1) / oneOverS

			if i := uint(x0i); i < uint(len(buf)) {
				buf[i] += int2ϕ(d * a0)
			} else if debugOutOfBounds {
				println("out of bounds #2")
			}

			if x1i == x0i+2 {
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += int2ϕ(d * (one - a0 - am))
				} else if debugOutOfBounds {
					println("out of bounds #3")
				}
			} else {
				a1 := ((oneAndAHalf - x0f) << ϕ) / oneOverS
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += int2ϕ(d * (a1 - a0))
				} else if debugOutOfBounds {
					println("out of bounds #4")
				}
				dTimesS := int2ϕ((d << (2 * ϕ)) / oneOverS)
				for xi := x0i + 2; xi < x1i-1; xi++ {
					if i := uint(xi); i < uint(len(buf)) {
						buf[i] += dTimesS
					} else if debugOutOfBounds {
						println("out of bounds #5")
					}
				}
				a2 := a1 + (int1ϕ(x1i-x0i-3)<<(2*ϕ))/oneOverS
				if i := uint(x1i - 1); i < uint(len(buf)) {
					buf[i] += int2ϕ(d * (one - a2 - am))
				} else if debugOutOfBounds {
					println("out of bounds #6")
				}
			}

			if i := uint(x1i); i < uint(len(buf)) {
				buf[i] += int2ϕ(d * am)
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
