// Package imaging makes the copies of a logo the bar draws, and draws
// TimePulse's own mark as pixels.
//
// A logo is uploaded once, a few thousand pixels across, and drawn at the left
// of the bar at a fraction of that. Handing every visitor the original is what
// the copies exist to stop: they are cut to the part chosen for the bar and
// scaled to the size it draws, once, when the logo is saved.
//
// The mark is the other half, and no logo ever stands in for it. The installed
// app's icons, the executable's and the favicon.ico a browser asks for by name
// are pictures rather than the SVG the page draws the mark from - so they are
// drawn here, from favicon.svg's own figures.
package imaging
