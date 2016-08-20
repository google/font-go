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

func floor(x float32) int { return int(math.Floor(float64(x))) }
func ceil(x float32) int  { return int(math.Ceil(float64(x))) }

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
	a    []float32
	last point
	w    int
	h    int
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

func accumulate(dst []uint8, src []float32) {
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
		dst[i] = uint8(255 * a)
	}
}

const debugOutOfBounds = false

func (z *rasterizer) drawLine(p, q point) {
	if p.y == q.y {
		return
	}
	dir := float32(1)
	if p.y > q.y {
		dir, p, q = -1, q, p
	}
	dxdy := (q.x - p.x) / (q.y - p.y)

	x := p.x
	if p.y < 0 {
		x -= p.y * dxdy
	}
	// TODO: floor instead of round to zero? Make this max(0, etc)? int instead of uint is more Go-like.
	y := int(p.y)
	yMax := ceil(q.y)
	if yMax > z.h {
		yMax = z.h
	}
	for ; y < yMax; y++ {
		buf := z.a[y*z.w:]
		dy := min(float32(y+1), q.y) - max(float32(y), p.y)
		xNext := x + dxdy*dy
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
			if i := uint(x0i + 0); i < uint(len(buf)) {
				buf[i] += d - d*xmf
			} else if debugOutOfBounds {
				println("out of bounds #0")
			}
			if i := uint(x0i + 1); i < uint(len(buf)) {
				buf[i] += d * xmf
			} else if debugOutOfBounds {
				println("out of bounds #1")
			}
		} else {
			s := 1 / (x1 - x0)
			x0f := x0 - x0Floor
			oneMinusX0f := 1 - x0f
			a0 := 0.5 * s * oneMinusX0f * oneMinusX0f
			x1f := x1 - x1Ceil + 1
			am := 0.5 * s * x1f * x1f
			if i := uint(x0i); i < uint(len(buf)) {
				buf[i] += d * a0
			} else if debugOutOfBounds {
				println("out of bounds #2")
			}
			if x1i == x0i+2 {
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += d * (1 - a0 - am)
				} else if debugOutOfBounds {
					println("out of bounds #3")
				}
			} else {
				a1 := s * (1.5 - x0f)
				if i := uint(x0i + 1); i < uint(len(buf)) {
					buf[i] += d * (a1 - a0)
				} else if debugOutOfBounds {
					println("out of bounds #4")
				}
				for xi := x0i + 2; xi < x1i-1; xi++ {
					if i := uint(xi); i < uint(len(buf)) {
						buf[i] += d * s
					} else if debugOutOfBounds {
						println("out of bounds #5")
					}
				}
				a2 := a1 + s*float32(x1i-x0i-3)
				if i := uint(x1i - 1); i < uint(len(buf)) {
					buf[i] += d * (1 - a2 - am)
				} else if debugOutOfBounds {
					println("out of bounds #6")
				}
			}
			if i := uint(x1i); i < uint(len(buf)) {
				buf[i] += d * am
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
