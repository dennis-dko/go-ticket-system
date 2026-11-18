package imaging_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// The mark is drawn at the size asked for, and is the mark: TimePulse's T and P
// in white on a tile that runs from a lighter blue at the top to a darker one at
// the bottom, the bowl of the P open around a clock, and nothing outside the
// tile's rounded corners.
//
// Each point is named in the 560-unit box favicon.svg is drawn in. The one inside
// the bowl is the one that says the arcs are traced the right way round: an arc
// swept the wrong way fills the bowl, or cuts the ring.
func TestTheShippedMarkIsDrawnAtTheSizeAsked(t *testing.T) {
	for _, size := range []int{16, 192, 512} {
		encoded, err := imaging.MarkPNG(size)
		if err != nil {
			t.Fatalf("drawing the mark at %d: %v", size, err)
		}

		img, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("the mark at %d is not a PNG: %v", size, err)
		}

		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("the mark asked for at %d is %dx%d", size, b.Dx(), b.Dy())
		}

		// At sixteen pixels a corner of 29 units is under one pixel and the figures
		// run into each other, so only the size is asked of it.
		if size < 64 {
			continue
		}

		at := func(x, y float64) [4]uint32 {
			r, g, b, a := img.At(int(x*float64(size)/560), int(y*float64(size)/560)).RGBA()

			return [4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}
		}

		if corner := at(0, 0); corner[3] != 0 {
			t.Errorf("the corner of the %d mark is not transparent: %v", size, corner)
		}

		blue := func(where string, p [4]uint32) {
			if p[3] != 0xff || p[2] < 0x70 || p[2] <= p[1] || p[1] <= p[0] {
				t.Errorf("%s of the %d mark reads as rgba%v, want the tile's blue", where, size, p)
			}
		}

		white := func(where string, p [4]uint32) {
			if p[3] != 0xff || p[0] < 0xf0 || p[1] < 0xf0 || p[2] < 0xf0 {
				t.Errorf("%s of the %d mark reads as rgba%v, want white", where, size, p)
			}
		}

		top, middle, bottom := at(280, 30), at(40, 280), at(450, 530)

		blue("the tile above the T", top)
		blue("the tile left of the T", middle)
		blue("the tile below the P", bottom)

		if top[0]+top[1]+top[2] <= bottom[0]+bottom[1]+bottom[2]+40 {
			t.Errorf("the %d mark's tile is %v at the top and %v at the bottom, want it darker going down",
				size, top, bottom)
		}

		white("the bar of the T", at(200, 115))
		white("the ring of the P", at(376, 114))
		white("the clock's hub", at(374.78, 216.5))

		blue("the bowl of the P below the hub", at(330, 250))
	}
}
