package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// The two icons kept in the tree show the mark as it is drawn now.
//
// They are pictures, and a picture in the tree is the copy that goes on showing
// an old mark: the web interface's favicon.ico did exactly that, a 32-pixel
// hourglass made by hand in August that no command could make again. So each
// frame of each file is decoded and compared with what imaging.Mark draws at
// that size - pixels rather than bytes, because a newer Go may compress the same
// picture differently, and with a little room, because a platform that fuses a
// multiply and an add may land a shade away.
func TestTheIconsInTheTreeAreTheMarkAsItIsDrawnNow(t *testing.T) {
	for _, icon := range []struct {
		path    string
		sizes   []int
		command string
	}{
		{filepath.Join("..", "icon.ico"), icoSizes, "go run ./build/icon > build/icon.ico"},
		{filepath.Join("..", "..", "internal", "interface", "web", "asset", "favicon.ico"), faviconSizes,
			"go run ./build/icon -favicon > internal/interface/web/asset/favicon.ico"},
	} {
		kept, err := os.ReadFile(icon.path)
		if err != nil {
			t.Fatalf("reading %s: %v", icon.path, err)
		}

		frames := framesOf(t, kept)
		if len(frames) != len(icon.sizes) {
			t.Errorf("%s holds %d frames, want %d; run %s", icon.path, len(frames), len(icon.sizes), icon.command)

			continue
		}

		for i, size := range icon.sizes {
			drawn, err := imaging.Mark(size)
			if err != nil {
				t.Fatalf("drawing the mark at %d: %v", size, err)
			}

			if !alike(frames[i], drawn) {
				t.Errorf("the %dpx frame of %s is not the mark as it is drawn now; run %s", size, icon.path, icon.command)
			}
		}
	}
}

// framesOf decodes every PNG frame of an icon file, in the order it lists them.
func framesOf(t *testing.T, ico []byte) []image.Image {
	t.Helper()

	if len(ico) < 6 {
		t.Fatalf("an icon file of %d bytes", len(ico))
	}

	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	frames := make([]image.Image, 0, count)

	for i := range count {
		entry := ico[6+16*i : 6+16*(i+1)]
		length := binary.LittleEndian.Uint32(entry[8:12])
		offset := binary.LittleEndian.Uint32(entry[12:16])

		frame, err := png.Decode(bytes.NewReader(ico[offset : offset+length]))
		if err != nil {
			t.Fatalf("frame %d is not a PNG: %v", i, err)
		}

		frames = append(frames, frame)
	}

	return frames
}

// alike reports whether two pictures are the same size and no channel of any
// pixel is more than two apart.
func alike(a, b image.Image) bool {
	if a.Bounds().Size() != b.Bounds().Size() {
		return false
	}

	ab, bb := a.Bounds(), b.Bounds()

	for y := range ab.Dy() {
		for x := range ab.Dx() {
			r1, g1, b1, a1 := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			r2, g2, b2, a2 := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()

			for _, pair := range [][2]uint32{{r1, r2}, {g1, g2}, {b1, b2}, {a1, a2}} {
				if apart := int(pair[0]>>8) - int(pair[1]>>8); apart > 2 || apart < -2 {
					return false
				}
			}
		}
	}

	return true
}
