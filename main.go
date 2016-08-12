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
	"flag"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path"
)

var (
	dumpFlag    = flag.Bool("dump", false, "print the vector data instead of rasterizing to out.png")
	fontFlag    = flag.String("font", path.Join(os.Getenv("HOME"), "fonts/Roboto-Regular.ttf"), "font filename")
	glyphIDFlag = flag.Int("glyphid", 76, "glyph ID; for example 76 is 'g' from Roboto-Regular")
	ppemFlag    = flag.Float64("ppem", 42, "pixels per em")
)

func main() {
	flag.Parse()
	b, err := ioutil.ReadFile(*fontFlag)
	if err != nil {
		log.Fatal(err)
	}
	f, err := parse(b)
	if err != nil {
		log.Fatal(err)
	}

	data := f.glyphData(uint16(*glyphIDFlag))
	// TODO: use the overall font's bbox from the head table, not the glyph's bbox.
	dx, dy, transform := data.glyphSizeAndTransform(f.scale(float32(*ppemFlag)))
	if *dumpFlag {
		f.dumpGlyph(data, transform)
		return
	}

	z := newRasterizer(dx, dy)
	z.rasterize(f, data, transform)
	dst := image.NewAlpha(z.Bounds())
	dst.Pix = make([]byte, len(dst.Pix)+accumulatorSlop)
	if haveAccumulateSIMD {
		accumulateSIMD(dst.Pix, z.a[:z.w*z.h])
	} else {
		accumulate(dst.Pix, z.a[:z.w*z.h])
	}

	out, err := os.Create("out.png")
	if err != nil {
		log.Fatal(err)
	}
	err = png.Encode(out, dst)
	if err != nil {
		log.Fatal(err)
	}
}
