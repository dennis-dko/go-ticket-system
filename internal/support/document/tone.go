package document

// Tone is what a figure says with its colour on the screen it was read from: a
// balance in credit or in debt, a day quieter than the rest, something true and
// worth knowing.
//
// The screen sends the word rather than the colour. What a screen is drawn in is
// a fact about its theme, and the dark theme's shades are made for a dark ground
// - a pale grey for what is quieter, a tint that is nearly black - which is the
// reason the page keeps its own ink (Palette.resolve). So the page prints a tone
// in shades of its own, and they are the light theme's, the one drawn on white;
// TestTheTonesArePrintedInTheLightScreensShades holds them to the stylesheet.
//
// A tone this page does not know is printed as none, for the reason a colour it
// cannot parse is: it is not worth refusing a document over.
type Tone string

const (
	// Plus is a balance in credit and Minus one in debt, each in the bold face
	// the screen sets them in.
	Plus  Tone = "plus"
	Minus Tone = "minus"

	// Muted is quieter than what is around it: a day with hours booked on it and
	// no working time recorded, a rate not set yet. It is the grey the page
	// writes its captions in, as the screen writes both in one.
	Muted Tone = "muted"

	// Notice is something true and worth knowing, which the screen tints: a limit
	// of the Working Hours Act a day went past.
	Notice Tone = "notice"
)

// The shades a tone is printed in, which are the light screen's: the green of
// .plus, the danger colour .minus is drawn in, and the tint behind a notice as
// it is mixed on white.
var (
	plusInk    = colour{0x2e, 0x9e, 0x5b}
	minusInk   = colour{0xc0, 0x39, 0x2b}
	noticeFill = colour{0xf8, 0xea, 0xd4}
)

// bold says whether a figure of this tone is set in the bold face.
func (t Tone) bold() bool { return t == Plus || t == Minus }

// shade is how a figure is printed: in which ink and face, and on what, where it
// is filled at all.
type shade struct {
	ink    colour
	bold   bool
	fill   colour
	filled bool
}

// shadeOf is how a figure of tone is printed. Plain is the ink its kind of
// figure has when it says nothing: the body's for a cell, the accent for a
// figure of the summary.
func (ink inks) shadeOf(tone Tone, plain colour) shade {
	look := shade{ink: plain, bold: tone.bold()}

	switch tone {
	case Plus:
		look.ink = plusInk
	case Minus:
		look.ink = minusInk
	case Muted:
		look.ink = ink.muted
	case Notice:
		look.fill, look.filled = noticeFill, true
	}

	return look
}
