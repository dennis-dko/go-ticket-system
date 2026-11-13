package mobile

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dennis-dko/go-ticket-system/internal/interface/web"
	"github.com/dennis-dko/go-ticket-system/internal/support/i18n"
)

// sources are the app's Kotlin files, tests aside, by their path.
func sources(t *testing.T) map[string]string {
	t.Helper()

	found := map[string]string{}

	err := filepath.WalkDir(filepath.Join("src", "main", "kotlin"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}

		body, err := os.ReadFile(path)
		found[filepath.ToSlash(path)] = string(body)

		return err
	})
	if err != nil {
		t.Fatalf("reading the app's sources: %v", err)
	}

	if len(found) < 5 {
		t.Fatalf("found %d Kotlin files; the reading has stopped finding them", len(found))
	}

	return found
}

// lexed is a Kotlin file with its comments blanked out - so nothing a comment
// says is read as code - and the string literals it holds, each by the index of
// its opening quote. Walked once from the start, because a regular expression
// cannot tell the // in "http://" from a comment, nor a quote in a comment from
// one that opens a string.
func lexed(body string) (string, map[int]string) {
	code := []byte(body)
	literals := map[int]string{}
	blank := func(from, to int) {
		for i := from; i < to; i++ {
			if code[i] != '\n' {
				code[i] = ' '
			}
		}
	}

	for i := 0; i < len(body); {
		switch {
		case strings.HasPrefix(body[i:], "//"):
			end := strings.IndexByte(body[i:], '\n')
			if end < 0 {
				end = len(body) - i
			}

			blank(i, i+end)
			i += end
		case strings.HasPrefix(body[i:], "/*"):
			end, depth := i+2, 1
			for ; end < len(body) && depth > 0; end++ {
				switch {
				case strings.HasPrefix(body[end:], "/*"):
					depth++
					end++
				case strings.HasPrefix(body[end:], "*/"):
					depth--
					end++
				}
			}

			blank(i, end)
			i = end
		case strings.HasPrefix(body[i:], `"""`):
			end := strings.Index(body[i+3:], `"""`)
			if end < 0 {
				end = len(body) - i - 3
			}

			literals[i] = body[i+3 : i+3+end]
			i += end + 6
		case body[i] == '"' || body[i] == '\'':
			end := i + 1
			for end < len(body) && body[end] != body[i] {
				if body[end] == '\\' {
					end++
				}

				end++
			}

			if body[i] == '"' {
				literals[i] = body[i+1 : min(end, len(body))]
			}

			i = end + 1
		default:
			i++
		}
	}

	return string(code), literals
}

// source is the one Kotlin file of the app called name.
func source(t *testing.T, name string) string {
	t.Helper()

	for path, body := range sources(t) {
		if filepath.Base(path) == name {
			return body
		}
	}

	t.Fatalf("the app has no %s", name)

	return ""
}

// read is a file of the repository, by its path from here.
func read(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

// The app's two ways of asking a dictionary: text with the key and its English
// written out, and lookup with the key alone, for a sentence whose English is
// somebody else's. A key put together at run time - err.$code - is neither: it
// belongs to the page's families, which the page's tests hold to the server.
var (
	textCall   = regexp.MustCompile(`\btext\(\s*"((?:[^"\\$]|\\.)+)",\s*"((?:[^"\\]|\\.)*)"`)
	lookupCall = regexp.MustCompile(`\blookup\(\s*"((?:[^"\\$]|\\.)+)"\s*\)`)
)

// looked is a key the app looks up: the English it says where a dictionary has
// nothing - none for a lookup - and where it is looked up.
type looked struct {
	english string
	at      string
}

// lookups are the keys the app looks up.
func lookups(t *testing.T) map[string]looked {
	t.Helper()

	found := map[string]looked{}

	for path, body := range sources(t) {
		body, _ := lexed(body)

		for _, m := range textCall.FindAllStringSubmatchIndex(body, -1) {
			key, english := body[m[2]:m[3]], kotlinString(body[m[4]:m[5]])
			at := fmt.Sprintf("%s:%d", path, strings.Count(body[:m[0]], "\n")+1)

			if earlier, ok := found[key]; ok && earlier.english != "" && earlier.english != english {
				t.Errorf("%s says %s as %q, and %s as %q", earlier.at, key, earlier.english, at, english)
			}

			found[key] = looked{english: english, at: at}
		}

		for _, m := range lookupCall.FindAllStringSubmatchIndex(body, -1) {
			if key := body[m[2]:m[3]]; found[key].at == "" {
				found[key] = looked{at: fmt.Sprintf("%s:%d", path, strings.Count(body[:m[0]], "\n")+1)}
			}
		}
	}

	// A floor for a reading that has stopped finding anything, not a count: the
	// app says little of its own since it shows the web interface, which says the
	// rest - twenty-odd keys when that was decided, from over sixty before.
	if len(found) < 20 {
		t.Fatalf("found %d keys looked up; the reading has stopped finding them", len(found))
	}

	return found
}

// kotlinString is the text a Kotlin string literal's inside says.
func kotlinString(inside string) string {
	var out strings.Builder

	for i := 0; i < len(inside); i++ {
		if inside[i] != '\\' || i+1 == len(inside) {
			out.WriteByte(inside[i])

			continue
		}

		i++

		switch inside[i] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'u':
			if code, err := strconv.ParseUint(inside[i+1:min(i+5, len(inside))], 16, 32); err == nil {
				out.WriteRune(rune(code))
				i += 4
			}
		default:
			out.WriteByte(inside[i])
		}
	}

	return out.String()
}

// place is one of {0}, {1} ... in a sentence.
var place = regexp.MustCompile(`\{\d+\}`)

// places is which values a sentence takes, each once, in order.
func places(sentence string) []string {
	return slices.Compact(slices.Sorted(slices.Values(place.FindAllString(sentence, -1))))
}

// Every key the app looks up is in every dictionary and fills in the places its
// English does, and every mobile.* entry is one the app still looks up.
func TestEveryKeyTheAppLooksUpIsInEveryDictionary(t *testing.T) {
	used := lookups(t)

	for _, language := range i18n.Languages()[1:] {
		words := i18n.Dictionary(language)

		for _, key := range slices.Sorted(maps.Keys(used)) {
			sentence, ok := words[key]
			if !ok {
				t.Errorf("%s looks up %s, which %s has no words for", used[key].at, key, language)

				continue
			}

			if english := used[key].english; english != "" && !slices.Equal(places(english), places(sentence)) {
				t.Errorf("%s fills in %v in English and %v in %s", key, places(english), places(sentence), language)
			}
		}

		for _, key := range slices.Sorted(maps.Keys(words)) {
			if _, ok := used[key]; strings.HasPrefix(key, "mobile.") && !ok {
				t.Errorf("%s has %s, which the app no longer looks up", language, key)
			}
		}
	}
}

// What the app borrows it says in the English of whoever it borrows it from -
// the page's words, the i18n package's figures - so the two never put one thing
// two ways in English either.
func TestTheAppSaysWhatTheOthersSayInTheirEnglish(t *testing.T) {
	theirs := othersEnglish(t)

	for key, mine := range lookups(t) {
		if mine.english == "" || strings.HasPrefix(key, "mobile.") {
			continue
		}

		said, ok := theirs[key]
		if !ok {
			t.Errorf("%s borrows %s, which nothing else says in English", mine.at, key)

			continue
		}

		if !said[mine.english] {
			t.Errorf("%s says %s as %q, where the others say %q", mine.at, key, mine.english, slices.Sorted(maps.Keys(said)))
		}
	}
}

// othersEnglish is what the i18n package and the page say in English, by key.
func othersEnglish(t *testing.T) map[string]map[string]bool {
	t.Helper()

	said := map[string]map[string]bool{}
	say := func(key, english string) {
		if said[key] == nil {
			said[key] = map[string]bool{}
		}

		said[key][english] = true
	}

	for _, dir := range []string{"../../support/i18n"} {
		names, err := filepath.Glob(filepath.Join(filepath.FromSlash(dir), "*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, name := range names {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}

			file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}

			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}

				if selector, ok := call.Fun.(*ast.SelectorExpr); !ok || (selector.Sel.Name != "text" && selector.Sel.Name != "Text") {
					return true
				}

				if key, english := goString(call.Args[0]), goString(call.Args[1]); key != "" {
					say(key, english)
				}

				return true
			})
		}
	}

	// A sentence too long for one line is written as several literals joined
	// with +, and it is the whole of it the page says.
	script := read(t, "../web/asset/app.js")
	literal := regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)
	unescape := strings.NewReplacer(`\'`, `'`, `\\`, `\`)

	for _, m := range regexp.MustCompile(`\bt\(\s*'((?:[^'\\]|\\.)+)',\s*((?:'(?:[^'\\]|\\.)*'\s*\+?\s*)+)`).FindAllStringSubmatch(script, -1) {
		var english strings.Builder

		for _, part := range literal.FindAllStringSubmatch(m[2], -1) {
			english.WriteString(unescape.Replace(part[1]))
		}

		say(m[1], english.String())
	}

	markup := read(t, "../web/asset/index.html")
	for _, m := range regexp.MustCompile(`data-i18n="([^"]+)"[^>]*>([^<]*)<`).FindAllStringSubmatch(markup, -1) {
		say(m[1], html.UnescapeString(strings.TrimSpace(m[2])))
	}

	return said
}

// goString is what a Go expression says when it is a string written out, or
// time.DateOnly - the i18n package's English for layout.day - and nothing for
// anything else.
func goString(expr ast.Expr) string {
	if literal, ok := expr.(*ast.BasicLit); ok && literal.Kind == token.STRING {
		said, _ := strconv.Unquote(literal.Value)

		return said
	}

	if selector, ok := expr.(*ast.SelectorExpr); ok && selector.Sel.Name == "DateOnly" {
		return time.DateOnly
	}

	return ""
}

// Nothing the app says to somebody is written as a bare string, where a
// dictionary cannot reach it: every literal that reads as words is a key or the
// English handed to text.
func TestNoSentenceIsSaidAroundTheDictionaries(t *testing.T) {
	prose := regexp.MustCompile(`[A-Za-z]{2,} [A-Za-z]{2,}`)
	said := 0

	for path, body := range sources(t) {
		code, literals := lexed(body)

		handed := map[int]bool{}
		for _, m := range textCall.FindAllStringSubmatchIndex(code, -1) {
			handed[m[4]-1] = true
		}

		for at, literal := range literals {
			if !prose.MatchString(literal) {
				continue
			}

			if handed[at] {
				said++

				continue
			}

			t.Errorf("%s:%d says %q around the dictionaries; look it up with text",
				path, strings.Count(body[:at], "\n")+1, literal)
		}
	}

	// A floor, for the reason lookups gives one.
	if said < 8 {
		t.Fatalf("found %d sentences handed to text; the reading has stopped finding them", said)
	}
}

// The app writes a refusal's values the way the page's REFUSAL_VALUES says, and
// has a way of its own for every way that table names.
func TestTheAppWritesARefusalsValuesAsThePageDoes(t *testing.T) {
	body := source(t, "Refusal.kt")
	page := web.RefusalValues()

	if len(page) == 0 {
		t.Fatal("the page's REFUSAL_VALUES could not be read")
	}

	mine := map[string]map[int]string{}

	for _, m := range regexp.MustCompile(`"(\w+)" to mapOf\(([^)]*)\)`).FindAllStringSubmatch(body, -1) {
		values := map[int]string{}

		for _, v := range regexp.MustCompile(`(\d+) to "(\w+)"`).FindAllStringSubmatch(m[2], -1) {
			at, _ := strconv.Atoi(v[1])
			values[at] = v[2]
		}

		mine[m[1]] = values
	}

	if !maps.EqualFunc(mine, page, maps.Equal) {
		t.Errorf("the app writes the values of refusals as %v, the page as %v", mine, page)
	}

	ways := map[string]bool{}
	for _, m := range regexp.MustCompile(`"(\w+)" ->`).FindAllStringSubmatch(body, -1) {
		ways[m[1]] = true
	}

	for code, values := range page {
		for at, way := range values {
			if !ways[way] {
				t.Errorf("the page writes value %d of %s with %s, which the app has no way of its own for", at, code, way)
			}
		}
	}
}

// The app is opened by the address a new token's code holds: the scheme the
// server writes is the one the manifest registers and the one the app reads.
func TestTheAppAnswersToTheAddressATokensCodeHolds(t *testing.T) {
	scheme := ""

	file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash("../api/v1/rest/apitoken_handler.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.ValueSpec); ok && len(spec.Names) == 1 && spec.Names[0].Name == "connectScheme" && len(spec.Values) == 1 {
			scheme = goString(spec.Values[0])
		}

		return true
	})

	if scheme == "" {
		t.Fatal("the server's connectScheme could not be read")
	}

	if handler := read(t, "../api/v1/rest/apitoken_handler.go"); !strings.Contains(handler, `connectScheme + "://connect?"`) {
		t.Errorf("the server no longer writes %s://connect?, which is what the app is registered for", scheme)
	}

	registered := false

	for _, data := range regexp.MustCompile(`<data\b[^>]*/>`).FindAllString(read(t, "src/main/AndroidManifest.xml"), -1) {
		registered = registered || strings.Contains(data, `android:scheme="`+scheme+`"`) && strings.Contains(data, `android:host="connect"`)
	}

	if !registered {
		t.Errorf("the manifest does not register %s://connect", scheme)
	}

	if body := source(t, "Connection.kt"); !strings.Contains(body, `const val connectScheme = "`+scheme+`"`) || !strings.Contains(body, `uri.host != "connect"`) {
		t.Errorf("the app does not read %s://connect as the address of a connection", scheme)
	}
}

// The web interface is laid out in a WebView exactly as tall as its place. One
// whose height is WRAP_CONTENT - which AndroidView gives a view without layout
// params of its own - lays its page out in a viewport of no height, so the
// page's 100dvh comes to nothing: its opened menu and its tour were drawn as thin
// as their padding, under the cards they should have covered. Nothing here runs
// a WebView, so what is held is the line in the code that sets the height.
func TestTheWebViewIsAsTallAsItsPlace(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	fills := regexp.MustCompile(`\blayoutParams\s*=\s*ViewGroup\.LayoutParams\(\s*` +
		`ViewGroup\.LayoutParams\.MATCH_PARENT,\s*ViewGroup\.LayoutParams\.MATCH_PARENT\s*\)`)

	if !fills.MatchString(code) {
		t.Error("Web.kt does not give the WebView MATCH_PARENT layout params, so the page's 100dvh may be nothing")
	}
}

// The page puts away its card about installing it as an app when it is shown in
// the Android app, which is one already: there the card told somebody to look in
// a browser's menu that is not on the screen. The page knows the app by the
// bridge the app registers for saving files, so the name the page asks for is
// the name Web.kt registers.
func TestThePageKnowsTheAppByTheBridgeItRegisters(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	registered := regexp.MustCompile(`\bconst\s+val\s+filesInterface\s*=\s*"([A-Za-z]+)"`).FindStringSubmatch(code)
	if registered == nil {
		t.Fatal("Web.kt names no filesInterface, so the page cannot know it is in the app")
	}

	script := read(t, "../web/asset/app.js")

	// Each place the page behaves differently inside the app, and what it does
	// there when it no longer recognises the app.
	for function, otherwise := range map[string]string{
		"wireInstall":             "shows a card about installing the page as an app",
		"passkeysWithheldBecause": "says the phone's browser cannot use passkeys, where the app signs in with its token",
	} {
		start := strings.Index(script, "function "+function+"()")
		if start < 0 {
			t.Fatalf("app.js has no %s", function)
		}

		body := script[start:]
		body = body[:strings.Index(body, "\n}\n")]

		if !strings.Contains(body, "'"+registered[1]+"' in window") {
			t.Errorf("%s does not ask for %s, the bridge the app registers, so the app %s", function, registered[1], otherwise)
		}
	}
}

// The page tells the app when its session ended without its being signed out
// of, through the bridge the app registers for it - the app cannot see that for
// itself, and without it a token taken back left the app showing the page's own
// sign-in form, still holding a token the installation no longer knew. So the
// name the page tells is the name Web.kt registers.
func TestThePageTellsTheAppItsSessionEndedByTheBridgeItRegisters(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	registered := regexp.MustCompile(`\bconst\s+val\s+sessionInterface\s*=\s*"([A-Za-z]+)"`).FindStringSubmatch(code)
	if registered == nil {
		t.Fatal("Web.kt names no sessionInterface, so the page cannot tell the app its session ended")
	}

	if !strings.Contains(read(t, "../web/asset/app.js"), "window."+registered[1]+"?.ended()") {
		t.Errorf("app.js does not tell %s, the bridge the app registers, that its session ended, so a "+
			"token taken back leaves the app on the page's own sign-in form", registered[1])
	}
}

// A receiver the manifest does not name is never called, and nothing says so:
// the alarm or the broadcast it was written for arrives, and nothing in the app
// hears it. So every BroadcastReceiver in the app's sources is a receiver of its
// manifest.
func TestEveryReceiverOfTheAppIsInItsManifest(t *testing.T) {
	manifest := read(t, "src/main/AndroidManifest.xml")
	receiver := regexp.MustCompile(`\bclass\s+([A-Z][A-Za-z0-9]*)\s*(?:\([^)]*\))?\s*:\s*BroadcastReceiver\(\)`)

	for path, body := range sources(t) {
		code, _ := lexed(body)

		for _, found := range receiver.FindAllStringSubmatch(code, -1) {
			named := regexp.MustCompile(`<receiver\b[^>]*\bandroid:name="\.` + regexp.QuoteMeta(found[1]) + `"`)

			if !named.MatchString(manifest) {
				t.Errorf("%s declares the receiver %s, which the manifest does not name, so Android never calls it", path, found[1])
			}
		}
	}
}

// The screens before the web interface takes over are drawn in its colours,
// light and dark, so that they look like its own sign-in: Look.kt keeps them as
// Tones, and they are app.css's custom properties - the band across the card is
// the sign-in band's.
func TestTheAppsColoursAreThePagesColours(t *testing.T) {
	css := read(t, "../web/asset/app.css")
	look, _ := lexed(source(t, "Look.kt"))

	page := func(selector string) map[string]string {
		at := strings.Index(css, selector+" {")
		if at < 0 {
			t.Fatalf("app.css has no %s block", selector)
		}

		block := css[at : at+strings.Index(css[at:], "}")]
		found := map[string]string{}

		for _, m := range regexp.MustCompile(`--([a-z-]+):\s*(#[0-9a-fA-F]{6});`).FindAllStringSubmatch(block, -1) {
			found[m[1]] = strings.ToLower(m[2])
		}

		return found
	}

	app := func(name string) map[string]string {
		line := regexp.MustCompile(`val ` + name + ` = Tones\(([^\n]*)\)`).FindStringSubmatch(look)
		if line == nil {
			t.Fatalf("Look.kt has no Tones called %s", name)
		}

		found := map[string]string{}

		for _, m := range regexp.MustCompile(`(\w+) = Color\(0xFF([0-9A-Fa-f]{6})\)`).FindAllStringSubmatch(line[1], -1) {
			found[m[1]] = "#" + strings.ToLower(m[2])
		}

		return found
	}

	properties := map[string]string{
		"bg": "bg", "surface": "surface", "border": "border", "text": "text", "muted": "muted",
		"accent": "accent", "accentText": "accent-text", "danger": "danger",
	}

	for _, theme := range []struct{ tones, selector string }{{"light", ":root"}, {"dark", `:root[data-theme="dark"]`}} {
		mine, theirs := app(theme.tones), page(theme.selector)

		for field, property := range properties {
			if mine[field] == "" || mine[field] != theirs[property] {
				t.Errorf("the app's %s %s is %q, where app.css's --%s is %q", theme.tones, field, mine[field], property, theirs[property])
			}
		}
	}

	band := regexp.MustCompile(`\.login-band \{[^}]*background: (#[0-9a-fA-F]{6})`).FindStringSubmatch(css)
	mine := regexp.MustCompile(`val band = Color\(0xFF([0-9A-Fa-f]{6})\)`).FindStringSubmatch(look)

	if band == nil || mine == nil || "#"+strings.ToLower(mine[1]) != strings.ToLower(band[1]) {
		t.Errorf("the app's band is %v, where the sign-in band's is %v", mine, band)
	}
}

// Every dictionary writes a day and a time in parts of a Go layout the app knows
// - the very parts Language.pattern reads, taken from it - because anything else is
// written out by the app as it stands, and 2.1.2006 would read as a date to
// nobody.
func TestEveryDictionaryWritesADayAndATimeInPartsTheAppReads(t *testing.T) {
	var parts []string

	for _, m := range regexp.MustCompile(`"(\d+)" to "[a-zA-Z]+"`).FindAllStringSubmatch(source(t, "Language.kt"), -1) {
		parts = append(parts, m[1])
	}

	if len(parts) < 6 {
		t.Fatalf("read %d parts of a layout from Language.pattern; the reading has stopped finding them", len(parts))
	}

	for _, language := range i18n.Languages()[1:] {
		for _, key := range []string{"layout.day", "layout.time"} {
			layout := i18n.Dictionary(language)[key]

			for rest := layout; rest != ""; {
				if at := slices.IndexFunc(parts, func(part string) bool { return strings.HasPrefix(rest, part) }); at >= 0 {
					rest = rest[len(parts[at]):]

					continue
				}

				r, size := utf8.DecodeRuneInString(rest)
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					t.Errorf("%s writes %s as %q, and the app knows no part of a layout at %q", language, key, layout, rest)

					break
				}

				rest = rest[size:]
			}
		}
	}
}

// The app is built with the dictionaries the server reads - the directory
// itself, not a copy of it - so a language added there is spoken by the app as
// well.
func TestTheAppCarriesTheDictionariesEveryoneElseReads(t *testing.T) {
	m := regexp.MustCompile(`addStaticSourceDirectory\("([^"]+)"\)`).FindStringSubmatch(read(t, "build.gradle.kts"))
	if m == nil {
		t.Fatal("the build no longer adds the dictionaries to the app's assets")
	}

	if got, want := filepath.Clean(filepath.FromSlash(m[1])), filepath.Join("..", "..", "support", "i18n", "locale"); got != want {
		t.Errorf("the app's dictionaries are copied from %s, and the application's are in %s", got, want)
	}

	for _, language := range i18n.Languages()[1:] {
		if _, err := os.Stat(filepath.Join(filepath.FromSlash(m[1]), language+".json")); err != nil {
			t.Errorf("the build copies no dictionary for %s: %v", language, err)
		}
	}
}
