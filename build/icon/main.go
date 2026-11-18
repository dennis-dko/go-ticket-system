package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// icoSizes are the frames Windows asks for.
var icoSizes = []int{16, 32, 48, 256}

// faviconSizes are the frames a browser reads /favicon.ico for: sixteen for the
// tab, and thirty-two where its pixels are dense.
var faviconSizes = []int{16, 32}

func main() {
	favicon := flag.Bool("favicon", false, "write the web interface's favicon.ico rather than the Windows icon")
	flag.Parse()

	sizes := icoSizes
	if *favicon {
		sizes = faviconSizes
	}

	if err := run(os.Stdout, sizes); err != nil {
		fmt.Fprintln(os.Stderr, "drawing the icon:", err)
		os.Exit(1)
	}
}

func run(out io.Writer, sizes []int) error {
	frames := make([][]byte, 0, len(sizes))

	for _, size := range sizes {
		encoded, err := imaging.MarkPNG(size)
		if err != nil {
			return fmt.Errorf("drawing %dpx: %w", size, err)
		}

		frames = append(frames, encoded)
	}

	return writeICO(out, sizes, frames)
}

// writeICO packs the frames into the container Windows reads.
//
// PNG inside rather than the older bitmap-with-mask: every Windows that this
// application supports reads it, and the 256 as a bitmap would be a quarter of a
// megabyte on its own.
func writeICO(out io.Writer, sizes []int, frames [][]byte) error {
	var buf bytes.Buffer

	// ICONDIR: reserved, type 1 (icon), how many.
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(frames)))

	const dirEntry = 16

	offset := 6 + dirEntry*len(frames)

	for i, frame := range frames {
		// 256 is written as 0: the field is one byte, and 256 does not fit in it.
		side := byte(sizes[i])
		if sizes[i] >= 256 {
			side = 0
		}

		buf.WriteByte(side)                                     // width
		buf.WriteByte(side)                                     // height
		buf.WriteByte(0)                                        // colours in the palette; none, this is truecolour
		buf.WriteByte(0)                                        // reserved
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // colour planes
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32)) // bits per pixel
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(frame)))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(offset))

		offset += len(frame)
	}

	for _, frame := range frames {
		buf.Write(frame)
	}

	_, err := out.Write(buf.Bytes())

	return err
}
