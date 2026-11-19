package imaging

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// Each copy of a logo is made at the size it is drawn.
//
// What is uploaded is a few thousand pixels across, and the bar draws a fraction
// of that. Handing every visitor the original is what the copies exist to stop.
func TestALogoIsMadeInTheSizeEachPlaceDraws(t *testing.T) {
	logo := wordmark(t, 2776, 1299)

	for name, size := range map[string]struct{ width, height int }{
		"the header": {HeaderWidth, HeaderHeight},
		"the banner": {BannerWidth, BannerHeight},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := Fit(logo, Crop{}, size.width, size.height)
			if err != nil {
				t.Fatalf("converting: %v", err)
			}

			img := decode(t, out)
			bounds := img.Bounds()

			if bounds.Dx() > size.width || bounds.Dy() > size.height {
				t.Errorf("%dx%d does not fit in %dx%d",
					bounds.Dx(), bounds.Dy(), size.width, size.height)
			}

			// Smaller than what was uploaded, which is the whole point: a wordmark
			// of a few hundred kilobytes was being sent to draw a 40px mark.
			if len(out) >= len(logo) {
				t.Errorf("the %s version is %d characters against the original's %d",
					name, len(out), len(logo))
			}

			if !drawnOn(img) {
				t.Error("nothing was drawn")
			}
		})
	}
}

// A copy keeps the logo's proportions and is not padded out to its box.
//
// The bar places the logo itself, so a transparent margin baked into the image
// would only fight it for room.
func TestNoCopyIsPadded(t *testing.T) {
	logo := wordmark(t, 2000, 500)

	for name, box := range map[string]struct{ width, height int }{
		"the header": {HeaderWidth, HeaderHeight},
		"the banner": {BannerWidth, BannerHeight},
	} {
		copied := decode(t, mustFit(t, logo, box.width, box.height))

		// Four to one in, four to one out: nothing is cropped and nothing added.
		ratio := float64(copied.Bounds().Dx()) / float64(copied.Bounds().Dy())
		if ratio < 3.5 || ratio > 4.5 {
			t.Errorf("%s's copy is %dx%d, a ratio of %.2f against the logo's 4",
				name, copied.Bounds().Dx(), copied.Bounds().Dy(), ratio)
		}
	}
}

// A mark smaller than the box is left alone rather than blown up.
//
// Enlarging is the one thing scaling cannot do well: what comes out is the same
// picture, softer, in a larger file. The box is a ceiling.
func TestASmallMarkIsNotEnlarged(t *testing.T) {
	small := wordmark(t, 32, 32)

	copied := decode(t, mustFit(t, small, BannerWidth, BannerHeight))

	if copied.Bounds().Dx() != 32 || copied.Bounds().Dy() != 32 {
		t.Errorf("a 32px mark came out at %dx%d", copied.Bounds().Dx(), copied.Bounds().Dy())
	}
}

// Anything that is not one of the two formats a logo may be is refused.
func TestOnlyTheTwoFormatsAreConverted(t *testing.T) {
	for name, uri := range map[string]string{
		"an SVG":           "data:image/svg+xml;base64,PHN2Zy8+",
		"a page":           "data:text/html;base64,PGh0bWw+",
		"not encoded":      "data:image/png,plain",
		"not a URI at all": "hello",
	} {
		if _, err := Fit(uri, Crop{}, BannerWidth, BannerHeight); err == nil {
			t.Errorf("%s was converted", name)
		}
	}
}

func mustFit(t *testing.T, logo string, width, height int) string {
	t.Helper()

	out, err := Fit(logo, Crop{}, width, height)
	if err != nil {
		t.Fatalf("converting: %v", err)
	}

	return out
}

func decode(t *testing.T, dataURI string) image.Image {
	t.Helper()

	_, encoded, _ := strings.Cut(dataURI, ",")

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("the result is not base64: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("the result is not an image: %v", err)
	}

	return img
}

// wordmark is a logo shaped like the ones installations upload: a mark on the
// left, a band of text beside it, transparent everywhere else.
func wordmark(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			dx, dy := x-width/8, y-height/2

			switch {
			case dx*dx+dy*dy < (height/3)*(height/3):
				img.Set(x, y, color.RGBA{R: 205, G: 220, A: 255})
			case x > width/2 && y > height/3 && y < 2*height/3:
				img.Set(x, y, color.RGBA{A: 255})
			}
		}
	}

	var out bytes.Buffer

	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

func drawnOn(img image.Image) bool {
	return !drawnBounds(img).Empty()
}

// drawnBounds is the smallest rectangle holding everything that is not
// transparent.
func drawnBounds(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	found := image.Rectangle{Min: bounds.Max, Max: bounds.Min}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, alpha := img.At(x, y).RGBA(); alpha == 0 {
				continue
			}

			found.Min.X = min(found.Min.X, x)
			found.Min.Y = min(found.Min.Y, y)
			found.Max.X = max(found.Max.X, x+1)
			found.Max.Y = max(found.Max.Y, y+1)
		}
	}

	if found.Min.X > found.Max.X {
		return image.Rectangle{}
	}

	return found
}

// Only the part of the logo chosen for the bar is used.
//
// A wordmark with a symbol at one end can read better as the symbol alone, and
// nobody can guess which part is worth showing, so it is chosen.
func TestOnlyTheChosenPartIsUsed(t *testing.T) {
	// Left half green, right half black, so which half came through is a question
	// about a colour rather than about a size.
	logo := halves(t, 400, 200)

	left, err := Fit(logo, Crop{X: 0, Y: 0, W: 0.5, H: 1}, 100, 100)
	if err != nil {
		t.Fatalf("cropping to the left: %v", err)
	}

	if got := middleColour(t, left); got != (colours{green: true}) {
		t.Errorf("the left half came out as %+v", got)
	}

	right, err := Fit(logo, Crop{X: 0.5, Y: 0, W: 0.5, H: 1}, 100, 100)
	if err != nil {
		t.Fatalf("cropping to the right: %v", err)
	}

	if got := middleColour(t, right); got != (colours{black: true}) {
		t.Errorf("the right half came out as %+v", got)
	}
}

// A crop that makes no sense is the whole image rather than nothing.
//
// It arrives from a browser, so it can be anything: a stray drag, a rounding
// error on a small image, a value somebody typed into the request by hand. A logo
// shown complete is never wrong, only sometimes small - and nothing here should
// be able to leave the bar with an empty picture.
func TestAnImpossibleCropIsTheWholeImage(t *testing.T) {
	logo := halves(t, 400, 200)

	whole := decode(t, mustFit(t, logo, 100, 100))

	for name, crop := range map[string]Crop{
		"nothing at all":    {X: 0.5, Y: 0.5, W: 0, H: 0},
		"outside the image": {X: 4, Y: 4, W: 1, H: 1},
		"negative":          {X: -1, Y: -1, W: -1, H: -1},
		"vanishingly small": {X: 0.5, Y: 0.5, W: 0.0000001, H: 0.0000001},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := Fit(logo, crop, 100, 100)
			if err != nil {
				t.Fatalf("converting: %v", err)
			}

			got := decode(t, out)

			if got.Bounds() != whole.Bounds() {
				t.Errorf("%s produced %v rather than the whole image %v",
					name, got.Bounds(), whole.Bounds())
			}
		})
	}
}

type colours struct{ green, black bool }

// middleColour is what is in the middle of a converted image, which is what says
// which part of the original it was made from.
func middleColour(t *testing.T, dataURI string) colours {
	t.Helper()

	img := decode(t, dataURI)
	bounds := img.Bounds()

	r, g, b, _ := img.At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2).RGBA()

	return colours{
		green: g > r && g > b,
		black: r < 0x4000 && g < 0x4000 && b < 0x4000,
	}
}

// halves is an image whose left half is green and right half is black.
func halves(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			if x < width/2 {
				img.Set(x, y, color.RGBA{G: 220, A: 255})
			} else {
				img.Set(x, y, color.RGBA{A: 255})
			}
		}
	}

	var out bytes.Buffer

	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

// A part chosen in a shape the box does not have is fitted into it, not
// stretched to fill it.
//
// The part is chosen freely - any corner, any shape - so neither a square nor a
// strip has the proportions of the box it is going into. Filling the box would
// mean distorting the logo, which is the one thing an instance's mark must never
// come out as. So it is scaled until it fits and left at its own proportions.
func TestAPartOfAnyShapeIsFittedRatherThanStretched(t *testing.T) {
	logo := wordmark(t, 2000, 500)

	for name, part := range map[string]struct {
		crop            Crop
		lowest, highest float64
	}{
		// A square out of a four-to-one wordmark.
		"a square": {crop: Crop{X: 0.4, Y: 0, W: 0.25, H: 1}, lowest: 0.9, highest: 1.1},
		// A twenty-to-one strip out of the same.
		"a strip": {crop: Crop{X: 0, Y: 0.4, W: 1, H: 0.2}, lowest: 18, highest: 22},
	} {
		copied := decode(t, mustFitCrop(t, logo, part.crop, BannerWidth, BannerHeight))

		if ratio := float64(copied.Bounds().Dx()) / float64(copied.Bounds().Dy()); ratio < part.lowest ||
			ratio > part.highest {
			t.Errorf("%s came out of the banner's box at %dx%d, a ratio of %.2f",
				name, copied.Bounds().Dx(), copied.Bounds().Dy(), ratio)
		}

		if copied.Bounds().Dx() > BannerWidth || copied.Bounds().Dy() > BannerHeight {
			t.Errorf("%s came out at %dx%d, outside the %dx%d box", name,
				copied.Bounds().Dx(), copied.Bounds().Dy(), BannerWidth, BannerHeight)
		}
	}
}

func mustFitCrop(t *testing.T, logo string, crop Crop, width, height int) string {
	t.Helper()

	out, err := Fit(logo, crop, width, height)
	if err != nil {
		t.Fatalf("converting: %v", err)
	}

	return out
}
