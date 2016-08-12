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
	"fmt"
	"image"
	"math"

	"golang.org/x/image/math/f32"
)

// Flags for simple (non-compound) glyphs.
const (
	flagOnCurve      = 1 << 0 // 0x0001
	flagXShortVector = 1 << 1 // 0x0002
	flagYShortVector = 1 << 2 // 0x0004
	flagRepeat       = 1 << 3 // 0x0008

	// The same flag bits are overloaded to have two meanings, dependent on the
	// value of the flag{X,Y}ShortVector bits.
	flagPositiveXShortVector = 1 << 4 // 0x0010
	flagThisXIsSame          = 1 << 4 // 0x0010
	flagPositiveYShortVector = 1 << 5 // 0x0020
	flagThisYIsSame          = 1 << 5 // 0x0020
)

// Flags for compound glyphs.
const (
	flagArg1And2AreWords   = 1 << 0  // 0x0001
	flagArgsAreXYValues    = 1 << 1  // 0x0002
	flagRoundXYToGrid      = 1 << 2  // 0x0004
	flagWeHaveAScale       = 1 << 3  // 0x0008
	flagUnused4            = 1 << 4  // 0x0010
	flagMoreComponents     = 1 << 5  // 0x0020
	flagWeHaveAnXAndYScale = 1 << 6  // 0x0040
	flagWeHaveATwoByTwo    = 1 << 7  // 0x0080
	flagWeHaveInstructions = 1 << 8  // 0x0100
	flagUseMyMetrics       = 1 << 9  // 0x0200
	flagOverlapCompound    = 1 << 10 // 0x0400
)

func i16(b []byte, i int) int16 {
	return int16(uint16(b[i+0])<<8 | uint16(b[i+1])<<0)
}

func u16(b []byte, i int) uint16 {
	return uint16(b[i+0])<<8 | uint16(b[i+1])<<0
}

func u32(b []byte, i int) uint32 {
	return uint32(b[i+0])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3])<<0
}

func parse(b []byte) (*Font, error) {
	if len(b) < 12 {
		return nil, fmt.Errorf("font-go: invalid font")
	}
	n := int(u16(b, 4))
	if len(b) < 12+n*16 {
		return nil, fmt.Errorf("font-go: invalid font")
	}
	f := &Font{}
	for i, n := 0, int(u16(b, 4)); i < n; i++ {
		header := b[12+16*(i+0) : 12+16*(i+1)]
		offset := u32(header, 8)
		length := u32(header, 12)
		table := b[offset : offset+length] // TODO: bounds check.

		switch string(header[:4]) {
		case "glyf":
			f.glyf = glyf(table)
		case "head":
			f.head = head(table)
		case "loca":
			f.loca = loca(table)
		case "maxp":
			f.maxp = maxp(table) // TODO: check len(table) vs minimum.
		}
	}
	return f, nil
}

type Font struct {
	glyf glyf
	head head
	loca loca
	maxp maxp
}

func (f *Font) dumpGlyph(b glyphData, transform f32.Aff3) {
	g := b.glyphIter()
	if g.compoundGlyph() {
		for g.nextSubGlyph() {
			f.dumpGlyph(f.glyphData(g.subGlyphID), concat(&transform, &g.subTransform))
		}
		return
	}

	for g.nextContour() {
		fmt.Println("---")
		if false {
			// Explicit points only.
			for g.nextPoint() {
				fmt.Println(g.x, g.y, g.on)
			}
		} else {
			// Include implicit points and transform.
			for g.nextSegment() {
				fmt.Printf("%d\t%v\t%v\n", g.seg.op,
					mul(&transform, g.seg.p),
					mul(&transform, g.seg.q),
				)
			}
		}
	}
}

func (f *Font) scale(ppem float32) float32 {
	return ppem / float32(f.head.unitsPerEm())
}

func (f *Font) glyphData(glyphID uint16) glyphData {
	if int(glyphID) >= f.maxp.numGlyphs() {
		return nil
	}
	lo, hi := f.loca.glyfRange(glyphID, f.head.indexToLocFormat())
	if lo >= hi || hi-lo < 10 || hi > uint32(len(f.glyf)) {
		return nil
	}
	return glyphData(f.glyf[lo:hi])
}

type glyphData []byte

func (b glyphData) glyphSizeAndTransform(scale float32) (width, height int, t f32.Aff3) {
	if b == nil {
		return 0, 0, f32.Aff3{}
	}
	s := float64(scale)
	bbox := image.Rectangle{
		Min: image.Point{
			X: int(math.Floor(+s * float64(i16(b, 2)))),
			Y: int(math.Floor(-s * float64(i16(b, 8)))),
		},
		Max: image.Point{
			X: int(math.Ceil(+s * float64(i16(b, 6)))),
			Y: int(math.Ceil(-s * float64(i16(b, 4)))),
		},
	}
	return bbox.Dx(), bbox.Dy(), f32.Aff3{
		+scale, 0, -float32(bbox.Min.X),
		0, -scale, -float32(bbox.Min.Y),
	}
}

func (b glyphData) glyphIter() glyphIter {
	if b == nil {
		return glyphIter{}
	}
	nContours := int32(i16(b, 0))
	if nContours < 0 {
		if nContours != -1 {
			// Negative values other than -1 are invalid.
			return glyphIter{}
		}
		// We have a compound glyph.
		return glyphIter{
			data:      b,
			endIndex:  initialIndex,
			nContours: nContours,
		}
	}

	// We have a simple glyph.
	index := 10 + 2*int(nContours)
	if index > len(b) {
		return glyphIter{}
	}
	// The +1 for nPoints is because the np index in the file format is
	// inclusive, but Go's slice[:index] semantics are exclusive.
	nPoints := 1 + int(u16(b, index-2))

	// Skip the hinting instructions.
	if index+2 > len(b) {
		return glyphIter{}
	}
	insnLen := int(u16(b, index))
	index += 2 + insnLen
	if index > len(b) {
		return glyphIter{}
	}

	flagIndex := index
	xDataLen := 0
	yDataLen := 0
	for i := 0; ; {
		if i > nPoints {
			return glyphIter{}
		}
		if i == nPoints {
			break
		}

		// TODO: bounds checking inside this block.
		repeatCount := 1
		flag := b[index]
		index++
		if flag&flagRepeat != 0 {
			repeatCount += int(b[index])
			index++
		}

		xSize := 0
		if flag&flagXShortVector != 0 {
			xSize = 1
		} else if flag&flagThisXIsSame == 0 {
			xSize = 2
		}
		xDataLen += xSize * repeatCount

		ySize := 0
		if flag&flagYShortVector != 0 {
			ySize = 1
		} else if flag&flagThisYIsSame == 0 {
			ySize = 2
		}
		yDataLen += ySize * repeatCount

		i += repeatCount
	}

	if index+xDataLen+yDataLen > len(b) {
		return glyphIter{}
	}
	return glyphIter{
		data:      b,
		endIndex:  initialIndex,
		flagIndex: flagIndex,
		xIndex:    index,
		yIndex:    index + xDataLen,
		nContours: nContours,
		// The -1 is because the contour-end index in the file format is
		// inclusive, but Go's slice[:index] semantics are exclusive.
		prevEnd: -1,
	}
}

type glyf []byte

type head []byte

func (b head) indexToLocFormat() int { return int(u16(b, 50)) }
func (b head) unitsPerEm() int       { return int(u16(b, 18)) }

type loca []byte

func (b loca) glyfRange(glyphID uint16, indexToLocFormat int) (lo, hi uint32) {
	// TODO: bounds checking throughout this method.
	if indexToLocFormat == 0 {
		lo = 2 * uint32(u16(b, 2*int(glyphID)+0))
		hi = 2 * uint32(u16(b, 2*int(glyphID)+2))
	} else {
		lo = u32(b, 4*int(glyphID)+0)
		hi = u32(b, 4*int(glyphID)+4)
	}
	return lo, hi
}

type maxp []byte

func (b maxp) numGlyphs() int { return int(u16(b, 4)) }

const initialIndex = 10

type glyphIter struct {
	data []byte

	// Various indices into the data slice.
	//
	// For simple glyphs, endIndex points to the uint16 that is the inclusive
	// point index of the current contour's end.
	//
	// For compound glyphs, endIndex points to the next sub-glyph and the other
	// xxxIndex fields are unused.
	endIndex  int
	flagIndex int
	xIndex    int
	yIndex    int

	nContours int32 // -1 for compound glyphs.
	c         int32
	nPoints   int32
	p         int32
	prevEnd   int32

	// Explicit points.
	x, y    int16
	on      bool
	flag    uint8
	repeats uint8

	// Segments, including implicit points.
	seg                segment
	firstOnCurve       point
	firstOffCurve      point
	lastOffCurve       point
	firstOnCurveValid  bool
	firstOffCurveValid bool
	lastOffCurveValid  bool
	closing            bool
	allDone            bool

	// Sub-glyphs.
	subGlyphID   uint16
	subTransform f32.Aff3
}

func (g *glyphIter) compoundGlyph() bool { return g.nContours < 0 }

func (g *glyphIter) nextContour() (ok bool) {
	if g.c == g.nContours {
		return false
	}
	g.c++

	end := int32(u16(g.data, g.endIndex)) // TODO: bounds checking.
	g.endIndex += 2
	g.nPoints = end - g.prevEnd
	g.p = 0
	g.prevEnd = end

	g.firstOnCurveValid = false
	g.firstOffCurveValid = false
	g.lastOffCurveValid = false
	g.closing = false
	g.allDone = false

	return true
}

func (g *glyphIter) nextPoint() (ok bool) {
	// TODO: bounds checking throughout this method.
	if g.p == g.nPoints {
		return false
	}
	g.p++

	if g.repeats > 0 {
		g.repeats--
	} else {
		g.flag = g.data[g.flagIndex]
		g.flagIndex++
		if g.flag&flagRepeat != 0 {
			g.repeats = g.data[g.flagIndex]
			g.flagIndex++
		}
	}

	if g.flag&flagXShortVector != 0 {
		if g.flag&flagPositiveXShortVector != 0 {
			g.x += int16(g.data[g.xIndex])
		} else {
			g.x -= int16(g.data[g.xIndex])
		}
		g.xIndex += 1
	} else if g.flag&flagThisXIsSame == 0 {
		g.x += i16(g.data, g.xIndex)
		g.xIndex += 2
	}

	if g.flag&flagYShortVector != 0 {
		if g.flag&flagPositiveYShortVector != 0 {
			g.y += int16(g.data[g.yIndex])
		} else {
			g.y -= int16(g.data[g.yIndex])
		}
		g.yIndex += 1
	} else if g.flag&flagThisYIsSame == 0 {
		g.y += i16(g.data, g.yIndex)
		g.yIndex += 2
	}

	g.on = g.flag&flagOnCurve != 0
	return true
}

func (g *glyphIter) nextSegment() (ok bool) {
	for {
		if g.closing {
			if g.allDone {
				return false
			}

			switch {
			case !g.firstOffCurveValid && !g.lastOffCurveValid:
				g.allDone = true
				g.seg = segment{op: lineTo, p: g.firstOnCurve}
			case !g.firstOffCurveValid && g.lastOffCurveValid:
				g.allDone = true
				g.seg = segment{op: quadTo, p: g.lastOffCurve, q: g.firstOnCurve}
			case g.firstOffCurveValid && !g.lastOffCurveValid:
				g.allDone = true
				g.seg = segment{op: quadTo, p: g.firstOffCurve, q: g.firstOnCurve}
			case g.firstOffCurveValid && g.lastOffCurveValid:
				g.lastOffCurveValid = false
				g.seg = segment{
					op: quadTo,
					p:  g.lastOffCurve,
					q:  midPoint(g.lastOffCurve, g.firstOffCurve),
				}
			}
			return true
		}

		if !g.nextPoint() {
			g.closing = true
			continue
		}

		p := point{float32(g.x), float32(g.y)}
		if !g.firstOnCurveValid {
			if g.on {
				g.firstOnCurve = p
				g.firstOnCurveValid = true
				g.seg = segment{op: moveTo, p: p}
				return true
			} else if !g.firstOffCurveValid {
				g.firstOffCurve = p
				g.firstOffCurveValid = true
				continue
			} else {
				midp := midPoint(g.firstOffCurve, p)
				g.firstOnCurve = midp
				g.firstOnCurveValid = true
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				g.seg = segment{op: moveTo, p: midp}
				return true
			}

		} else if !g.lastOffCurveValid {
			if !g.on {
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				continue
			} else {
				g.seg = segment{op: lineTo, p: p}
				return true
			}

		} else {
			if !g.on {
				midp := midPoint(g.lastOffCurve, p)
				g.seg = segment{op: quadTo, p: g.lastOffCurve, q: midp}
				g.lastOffCurve = p
				g.lastOffCurveValid = true
				return true
			} else {
				g.seg = segment{op: quadTo, p: g.lastOffCurve, q: p}
				g.lastOffCurveValid = false
				return true
			}
		}
	}
}

func (g *glyphIter) nextSubGlyph() (ok bool) {
	// TODO: bounds checking throughout this method.
	if g.endIndex < 0 {
		return false
	}

	i, data := g.endIndex, g.data
	flags := u16(data, i+0)
	if flags&flagArgsAreXYValues == 0 {
		// TODO: handle this case, if it's ever used by real fonts.
		g.endIndex = -1
		return false
	}
	g.subGlyphID = u16(data, i+2)
	i += 4

	g.subTransform = f32.Aff3{
		1, 0, 0,
		0, 1, 0,
	}
	if flags&flagArg1And2AreWords != 0 {
		g.subTransform[2] = float32(i16(data, i+0))
		g.subTransform[5] = float32(i16(data, i+2))
		i += 4
	} else {
		g.subTransform[2] = float32(int8(data[i+0]))
		g.subTransform[5] = float32(int8(data[i+1]))
		i += 2
	}

	if flags&flagWeHaveAScale != 0 {
		t := float32(i16(data, i+0))
		g.subTransform[0] = t
		g.subTransform[4] = t
		i += 2
	} else if flags&flagWeHaveAnXAndYScale != 0 {
		g.subTransform[0] = float32(i16(data, i+0))
		g.subTransform[4] = float32(i16(data, i+2))
		i += 4
	} else if flags&flagWeHaveATwoByTwo != 0 {
		// TODO: check that it's 0,1,3,4 and not 0,3,1,4.
		g.subTransform[0] = float32(i16(data, i+0))
		g.subTransform[1] = float32(i16(data, i+2))
		g.subTransform[3] = float32(i16(data, i+4))
		g.subTransform[4] = float32(i16(data, i+6))
		i += 8
	}

	if flags&flagMoreComponents == 0 {
		// The remainder of the glyf data are hinting instructions, which we skip.
		g.endIndex = -1
	} else {
		g.endIndex = i
	}
	return true
}
