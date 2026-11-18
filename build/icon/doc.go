// Command icon draws the shipped mark and writes it as an icon file: the
// Windows one, build/icon.ico, or with -favicon the web interface's
// internal/interface/web/asset/favicon.ico.
//
// The .exe gets its icon from build/icon.ico, by way of goversioninfo and
// build/windows-resource.sh, and a browser asks for /favicon.ico by name. Both
// are binaries in the tree, copies of a picture that also exists as
// internal/interface/web/asset/favicon.svg - and a copy nobody can regenerate is
// a copy that silently keeps the old mark forever. That is exactly what happened
// to the tab icon: it was cached at an address that could not change, and went
// on showing a picture nobody had chosen; and favicon.ico was a hand-made
// hourglass until the mark changed and nothing could make it again.
//
// The mark is drawn by imaging.Mark, from favicon.svg's geometry in the same
// 560-unit box - the same drawing the server uses for an installed app's icons,
// so none of them can come to show a different picture. A test fails when the
// SVG and imaging.Mark part, and another when either file here is not the mark
// as it is drawn now.
//
//	go run ./build/icon > build/icon.ico
//	go run ./build/icon -favicon > internal/interface/web/asset/favicon.ico
//
// Four sizes for Windows, because it asks for four: 16 in the Explorer's detail
// view, 32 in the list and on the taskbar, 48 medium, 256 in the file's
// properties and wherever a preview is large. Two for the browser: 16 for the
// tab and 32 where pixels are dense. Each is drawn at its own size rather than
// scaled from one big one - a 256 shrunk to 16 loses the corners of the tile.
package main
