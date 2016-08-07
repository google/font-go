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
	ppemFlag    = flag.Int("ppem", 42, "pixels per em")
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

	if *dumpFlag {
		f.dumpGlyph(uint16(*glyphIDFlag), float32(*ppemFlag))
		return
	}

	z := newRasterizer(f.glyphSize(uint16(*glyphIDFlag), float32(*ppemFlag)))
	dst := image.NewAlpha(z.Bounds())
	dst.Pix = make([]byte, len(dst.Pix)+accumulatorSlop)
	z.rasterize(f, uint16(*glyphIDFlag), float32(*ppemFlag))

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
