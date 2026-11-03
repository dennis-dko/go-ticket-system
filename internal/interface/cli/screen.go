package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
)

// refreshEvery is how often the screen reads the day again while it is open.
// The stopwatch is the one the browser shows, so it may have been stopped
// there; half a minute is soon enough to notice that, and rare enough to cost
// the installation nothing.
const refreshEvery = 30 * time.Second

// screen is the client on a terminal: the stopwatch running live, what today has
// booked, a menu of the changes it can take, and a form for each of them.
//
// It makes those changes through the same session as the commands, so it can
// do nothing they cannot, and it says what it did in their words. It keeps the
// context it was opened with because Bubble Tea hands Update none, and every
// request the screen sends has to end when the program does.
type screen struct {
	ctx  context.Context
	s    *session
	host string

	width int

	// dark is what the terminal said its background is, and measured whether it
	// said anything; see darkness.
	dark, measured bool

	// title and banner are what the installation calls itself and says to
	// everybody, read with the day: what the page heads itself with, so the
	// terminal and the browser are headed alike.
	title, banner string

	// told is what about the owner's own time deserves saying, as the browser's
	// banners say it, and places where they keep public holidays.
	told   notices
	places holidayPlaces

	shown  snapshot
	readAt time.Time
	now    time.Time
	loaded bool

	// reading is a read of the day on its way, so the second hand does not ask
	// again every second while an installation is slow to answer. askedAt is
	// when the day was last asked for, answered or not: the half minute is
	// counted from there, because counted from the last answer it ran out for
	// good once an installation stopped answering, and from then on the screen
	// asked with every second.
	reading bool
	askedAt time.Time

	// busy is a change on its way, or one whose outcome has not been read yet.
	// Until it has, the screen shows the day as it was before the change, and a
	// key that acts on that would act on something that is no longer true.
	busy bool

	// news and failure are what the last change did, or why it was refused;
	// stale is why the last read failed, which leaves the day shown as it was.
	news    string
	failure string
	stale   string

	// cursor is the line of the menu somebody has moved to.
	cursor int

	// form is the one open, if any, and doing says which change it asks about.
	// asked holds its answers behind a pointer, because the form writes into it
	// while the screen itself is copied on every update.
	form  *huh.Form
	doing string
	asked *answers
}

// loaded is the day as the installation answered, what it calls itself and
// says, the notices, and the moment it was read. branded and noticed say whether
// those two could be read at all.
type loaded struct {
	shown    snapshot
	branding branding
	told     notices
	places   holidayPlaces
	at       time.Time
	err      error

	branded, noticed, placed bool
}

// done is what became of a change, in the sentence its command would print.
type done struct {
	said string
	err  error
}

// tick is the second hand.
type tick time.Time

// newScreen is the screen for one session, before anything has been read.
func newScreen(ctx context.Context, s *session) screen {
	host := s.api.base
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		host = u.Host
	}

	return screen{ctx: ctx, s: s, host: host, reading: true, askedAt: time.Now()}
}

// show runs the screen until it is closed.
func (s *session) show(ctx context.Context) error {
	ended, err := s.program(ctx).Run()

	return s.leave(ctx, ended, err)
}

// program is the screen as a program drawing on the session's output. The
// options are the tests' way in, since they have no terminal to give it.
func (s *session) program(ctx context.Context, options ...tea.ProgramOption) *tea.Program {
	options = append([]tea.ProgramOption{tea.WithContext(ctx), tea.WithOutput(s.out)}, options...)

	return tea.NewProgram(newScreen(ctx, s), options...)
}

// leave leaves a line in the terminal saying how the stopwatch was left: the
// screen takes its picture with it when it goes, and whoever looks back at the
// shell should still see that.
func (s *session) leave(ctx context.Context, ended tea.Model, err error) error {
	switch {
	case ctx.Err() != nil, errors.Is(err, tea.ErrInterrupted):
		return nil
	case err != nil:
		return fmt.Errorf("%s: %w", s.lang.text("track.screenFailed", "the screen could not be opened"), err)
	}

	if last, ok := ended.(screen); ok && last.loaded {
		s.say("%s", last.farewell(time.Now()))
	}

	return nil
}

// Init reads the day, starts the second hand, and asks the terminal whether it
// is dark, which decides the colours.
func (m screen) Init() tea.Cmd {
	return tea.Batch(m.load(), second(), tea.RequestBackgroundColor)
}

// ask reads the day again, and remembers when it was asked.
func (m screen) ask(now time.Time) (screen, tea.Cmd) {
	m.reading, m.askedAt = true, now

	return m, m.load()
}

// load reads the day again, what the installation calls itself and says, and
// the notices. A name, a banner or notices that could not be asked for leave the
// ones shown: nothing else depends on them, and an installation from before
// they were answered says nothing about them.
func (m screen) load() tea.Cmd {
	ctx, s := m.ctx, m.s

	return func() tea.Msg {
		shown, err := s.snapshot(ctx)
		if err != nil {
			return loaded{at: time.Now(), err: err}
		}

		named, nameErr := s.branding(ctx)
		told, toldErr := s.notices(ctx)
		places, placesErr := s.holidayPlaces(ctx)

		return loaded{shown: shown, branding: named, told: told, places: places, at: time.Now(),
			branded: nameErr == nil, noticed: toldErr == nil, placed: placesErr == nil}
	}
}

// second waits for the next second on the wall clock, so the stopwatch turns
// over when the clock on the wall does.
func second() tea.Cmd {
	return tea.Every(time.Second, func(at time.Time) tea.Msg { return tick(at) })
}

// Update answers a key, a second, or what the installation said.
func (m screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// From anywhere, a form included: escape is what puts a form away.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.form == nil {
			return m.press(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.BackgroundColorMsg:
		m.dark, m.measured = msg.IsDark(), true
	case tick:
		m.now = time.Time(msg)

		if !m.reading && m.now.Sub(m.askedAt) >= refreshEvery {
			next, read := m.ask(m.now)

			return next, tea.Batch(second(), read)
		}

		return m, second()
	case loaded:
		return m.read(msg)
	case done:
		m.news, m.failure = msg.said, ""
		if msg.err != nil {
			m.news, m.failure = "", msg.err.Error()
		}

		return m.ask(time.Now())
	}

	if m.form != nil {
		return m.fillIn(msg)
	}

	return m, nil
}

// read takes in the day as the installation answered. A failure keeps the day
// that was shown, under a line saying why: an installation briefly out of reach
// should not blank the screen of somebody looking at it.
//
// A stopwatch that has started or stopped offers other changes, so the cursor
// goes back to the first of them rather than staying on whichever change now
// stands where the one somebody moved to was.
//
// A language the account chose since is spoken from here on, unless one was
// named when the client was started - chosen in the browser or under Settings
// here, it reaches an open screen as a stopwatch stopped there does. The day is
// read again in it, since the installation's name and banner came in the old.
func (m screen) read(msg loaded) (screen, tea.Cmd) {
	m.reading, m.busy = false, false

	if msg.err != nil {
		m.stale = msg.err.Error()

		return m, nil
	}

	if chosen := msg.shown.language; !m.s.asked && chosen != m.s.lang.code() && slices.Contains(i18n.Languages(), chosen) {
		m.s = m.s.inLanguage(speaking(chosen))
		m = m.take(msg)

		return m.ask(time.Now())
	}

	return m.take(msg), nil
}

// take keeps what a read brought.
func (m screen) take(msg loaded) screen {
	if msg.shown.clock.Running != m.shown.clock.Running {
		m.cursor = 0
	}

	m.shown, m.readAt, m.loaded, m.stale = msg.shown, msg.at, true, ""

	if msg.branded {
		m.banner = msg.branding.banner

		if msg.branding.title != "" {
			m.title = msg.branding.title
		}
	}

	if msg.noticed {
		m.told = msg.told
	}

	if msg.placed {
		m.places = msg.places
	}

	if msg.at.After(m.now) {
		m.now = msg.at
	}

	return m
}

// choice is one line of the menu: what it does, what it is called, and the key
// that does it without moving there. The keys stay the same in every language,
// so a hand that knows them does not have to learn them again.
type choice struct{ key, doing, called string }

// choices are the menu: the changes the stopwatch can take as it stands, and
// reading the day again and closing the screen, which it can always take. A
// change it cannot take - stopping one that is not running - is not offered,
// and its key does nothing; none is offered before the day is read or while a
// change is on its way.
func (m screen) choices() []choice {
	l := m.s.lang

	var changes []choice

	switch {
	case !m.loaded || m.busy:
	case m.shown.clock.Running:
		changes = []choice{
			{"w", "switch", l.text("track.form.switch", "Switch to another project")},
			{"x", "stop", l.text("track.form.stop", "Stop the stopwatch")},
			{"b", "book", l.text("track.form.book", "Book time by hand")},
		}
	default:
		changes = []choice{
			{"s", "start", l.text("track.form.start", "Start the stopwatch")},
			{"b", "book", l.text("track.form.book", "Book time by hand")},
		}
	}

	// The account's own settings, once there is an account read to show them of.
	if m.loaded && !m.busy {
		changes = append(changes, choice{"p", "settings", l.text("track.menu.settings", "Settings")})
	}

	return append(changes,
		choice{"r", "refresh", l.text("track.menu.refresh", "Read the day again")},
		choice{"q", "quit", l.text("track.menu.quit", "Close the screen")})
}

// press answers a key while no form is open: a move through the menu, enter on
// the line moved to, or the key of a change.
func (m screen) press(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	choices := m.choices()
	at := min(m.cursor, len(choices)-1)

	switch k.String() {
	case "up", "k":
		m.cursor = max(at-1, 0)

		return m, nil
	case "down", "j":
		m.cursor = min(at+1, len(choices)-1)

		return m, nil
	case "enter":
		return m.choose(choices[at].doing)
	}

	for _, c := range choices {
		if c.key == k.String() {
			return m.choose(c.doing)
		}
	}

	return m, nil
}

// choose does what a line of the menu offers.
func (m screen) choose(doing string) (tea.Model, tea.Cmd) {
	switch doing {
	case "quit":
		return m, tea.Quit
	case "refresh":
		if m.reading {
			return m, nil
		}

		return m.ask(time.Now())
	}

	return m.open(doing)
}

// open puts up the form for a change, with what the last one said cleared away.
func (m screen) open(doing string) (tea.Model, tea.Cmd) {
	m.doing, m.news, m.failure = doing, "", ""
	m.asked = &answers{day: m.readAt.In(m.shown.zone).Format(time.DateOnly), stop: true}

	if doing == "settings" {
		m.asked = m.settingAnswers()
	}

	m.form = m.formFor(doing)

	return m, m.form.Init()
}

// fillIn hands a message to the open form, and sends the change once the form
// is answered.
func (m screen) fillIn(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)

	switch m.form.State {
	case huh.StateAborted:
		m.form = nil

		return m, nil
	case huh.StateCompleted:
		m.form = nil

		change := m.request()
		if change == nil {
			return m, nil
		}

		m.busy = true

		return m, change
	}

	return m, cmd
}

// elapsed is how long the stopwatch has run: what the installation measured
// when the day was read, and the time since on this machine's clock - so a
// clock that is set wrong here moves the second hand and not the hours.
func (m screen) elapsed() time.Duration {
	return time.Duration(m.shown.clock.ElapsedHours*float64(time.Hour)) + max(m.now.Sub(m.readAt), 0)
}

// farewell is the line the screen leaves in the terminal when it closes.
func (m screen) farewell(now time.Time) string {
	if now.After(m.now) {
		m.now = now
	}

	return m.shown.clockLine(m.s.lang, m.elapsed().Hours())
}

// contentWidth is how wide the screen's frame is drawn: the window less its
// margins, and no wider than a line can be read across.
func (m screen) contentWidth() int {
	if m.width == 0 {
		return 80
	}

	return min(max(m.width-4, 46), 92)
}

// inner is how wide what is inside the frame is: the frame less its border and
// its padding.
func (m screen) inner() int { return m.contentWidth() - 6 }

// View draws the screen in a frame: the installation's name, who and which
// build, where on the screen somebody is, and under it either the day and the
// menu or the form that is open.
func (m screen) View() tea.View {
	look := paletteFor(m.darkness())
	width := m.inner()
	rule := look.muted.Render(strings.Repeat("─", width))

	parts := []string{
		lipgloss.PlaceHorizontal(width, lipgloss.Center, look.title.Render(m.heading())),
		m.subtitle(look, width),
	}

	// Wrapped rather than cut short, as everything that says something is.
	wrapped := lipgloss.NewStyle().Width(width)

	if m.banner != "" {
		parts = append(parts, wrapped.Render(look.ask.Render(m.banner)))
	}

	parts = append(parts, rule, m.breadcrumb(look), rule)

	if m.form != nil {
		parts = append(parts, m.form.View(), m.formHelp())
	} else {
		for _, line := range m.noticeLines() {
			parts = append(parts, wrapped.Render(look.ask.Render("! "+line)))
		}

		parts = append(parts, m.stopwatch(look, width))

		if m.loaded {
			parts = append(parts, m.todayList(look, width))
		}

		parts = append(parts, rule, look.ask.Render(m.s.lang.text("track.menu.prompt", "Choose what to do next.")), m.menu(look, width))
	}

	// Wrapped rather than cut short: a reason comes at the end of its sentence.
	if line := m.statusLine(look); line != "" {
		parts = append(parts, wrapped.Render(line))
	}

	if m.form == nil {
		parts = append(parts, m.help(look))
	}

	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(look.frame).
		Padding(1, 2).Width(m.contentWidth())

	v := tea.NewView(lipgloss.NewStyle().Margin(1, 2).Render(frame.Render(strings.Join(parts, "\n\n"))))
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()

	return v
}

// heading is the installation's name, once it has been read, and the client's
// own until then.
func (m screen) heading() string {
	if m.title != "" {
		return m.title
	}

	return "go-time-recording"
}

// subtitle says who is signed in where, and which build of the client this is.
func (m screen) subtitle(look palette, width int) string {
	who := "@ " + m.host
	if m.shown.name != "" {
		who = "@ " + m.shown.name + " · " + m.host
	}

	b := m.s.build

	version := b.Version
	if version == "" {
		version = "dev"
	}

	built := m.s.lang.text("update.title", "Version") + " " + version
	if b.Commit != "" {
		built += " (" + strings.Join(nonEmpty(b.Commit, b.Day), " / ") + ")"
	}

	// On one line where both fit, and the build under who otherwise: a name cut
	// short to make room for a version says less than either.
	if lipgloss.Width(who)+2+lipgloss.Width(built) > width {
		return look.muted.Render(ansi.Truncate(who, width, "…") + "\n" + built)
	}

	return look.muted.Render(spread(width, who, built))
}

// nonEmpty is the words given that say something.
func nonEmpty(words ...string) []string {
	var said []string

	for _, w := range words {
		if w != "" {
			said = append(said, w)
		}
	}

	return said
}

// breadcrumb says where on the screen somebody is: today, and while a form is
// open the change it asks about and the step it is at, the last of them in the
// accent.
func (m screen) breadcrumb(look palette) string {
	l := m.s.lang
	steps := []string{l.text("cal.today", "Today")}

	if m.form != nil {
		steps = append(steps, m.changeName())

		if step := m.stepName(); step != "" {
			steps = append(steps, step)
		}
	}

	for i := range steps[:len(steps)-1] {
		steps[i] = look.muted.Render(steps[i])
	}

	steps[len(steps)-1] = look.title.Render(steps[len(steps)-1])

	return strings.Join(steps, look.muted.Render(" › "))
}

// changeName is what the menu calls the change the open form asks about.
func (m screen) changeName() string {
	for _, c := range m.choicesFor(m.doing) {
		if c.doing == m.doing {
			return c.called
		}
	}

	return m.doing
}

// choicesFor is the menu as it stood when doing was chosen from it: a switch and
// a stop were chosen from a running stopwatch's.
func (m screen) choicesFor(doing string) []choice {
	shown := m
	shown.loaded, shown.busy = true, false
	shown.shown.clock.Running = doing == "switch" || doing == "stop"

	return shown.choices()
}

// stepName is what the field the open form is asking about is called, or
// nothing for a form that asks one thing only.
func (m screen) stepName() string {
	l := m.s.lang

	switch m.form.GetFocusedField().GetKey() {
	case "project":
		return l.text("field.project", "Project")
	case "description":
		return l.text("field.description", "Description")
	case "hours":
		return l.text("field.hours", "Hours")
	case "day":
		return l.text("field.day", "Day")
	}

	return m.settingStep(m.form.GetFocusedField().GetKey())
}

// stopwatch is what the stopwatch runs on, since when, and for how long.
func (m screen) stopwatch(look palette, width int) string {
	l := m.s.lang

	switch {
	case !m.loaded && m.reading:
		return look.muted.Render(l.text("track.reading", "Reading {0} …", m.host))
	case !m.loaded:
		return look.muted.Render(l.text("track.nothingRead", "Nothing read from {0} yet.", m.host))
	}

	clock := m.shown.clock
	if !clock.Running || clock.StartedAt == nil {
		return lipgloss.NewStyle().Foreground(look.idle).Render("○  ") + l.text("track.notRunning", "The stopwatch is not running.")
	}

	on := l.nameOf(m.shown.projects, clock.ProjectID)
	if clock.Description != nil && *clock.Description != "" {
		on += look.muted.Render(" · " + *clock.Description)
	}

	left := lipgloss.NewStyle().Foreground(look.running).Render("●  ") + look.strong.Render(clockFace(m.elapsed())) + "   " + on
	right := look.muted.Render(l.text("track.since", "since {0}", clock.StartedAt.In(m.shown.zone).Format("15:04")))

	return spread(width, left, right)
}

// todayList is what today has booked, entry by entry, under the day and its
// total - the day by its date, since the breadcrumb above already says today.
func (m screen) todayList(look palette, width int) string {
	l := m.s.lang
	entries := m.shown.today
	day := l.weekday(m.readAt.In(m.shown.zone))

	heading := spread(width, look.strong.Render(day), look.strong.Render(l.hours(total(entries))))

	if len(entries) == 0 {
		return heading + "\n" + look.muted.Render(l.text("track.nothingYet", "Nothing booked yet."))
	}

	names := make([]string, len(entries))
	column := 0

	for i, e := range entries {
		names[i] = l.nameOf(m.shown.projects, e.ProjectID)
		column = max(column, lipgloss.Width(names[i]))
	}

	column = min(column, width/2)
	rows := []string{heading}
	hoursWidth := hoursColumn(l.hoursOf(entries)...)

	for i, e := range entries {
		name := ansi.Truncate(names[i], column, "…")
		row := fmt.Sprintf("   %*s  %s", hoursWidth, l.hours(e.DurationHours),
			name+strings.Repeat(" ", column-lipgloss.Width(name)))

		if e.Description != nil && *e.Description != "" {
			row += "  " + look.muted.Render(*e.Description)
		}

		rows = append(rows, ansi.Truncate(row, width, "…"))
	}

	return strings.Join(rows, "\n")
}

// menu is the choices, the one the cursor is on in the accent behind a >, each
// with its key at the end of its line.
func (m screen) menu(look palette, width int) string {
	choices := m.choices()
	at := min(m.cursor, len(choices)-1)
	rows := make([]string, len(choices))

	for i, c := range choices {
		line := "  " + c.called
		if i == at {
			line = look.title.Render("> " + c.called)
		}

		rows[i] = spread(width, line, look.key.Render(c.key))
	}

	return strings.Join(rows, "\n")
}

// statusLine says what the last change did, or why the day shown may be old.
func (m screen) statusLine(look palette) string {
	switch {
	case m.stale != "":
		return look.failed.Render("✗ " + m.stale)
	case m.failure != "":
		return look.failed.Render("✗ " + m.failure)
	case m.news != "":
		return look.ok.Render("✓ ") + m.news
	case m.busy:
		return look.muted.Render(m.s.lang.text("track.sending", "Sending …"))
	}

	return ""
}

// help names the keys that move through the menu, the way huh names its own
// under a form - so the menu and the forms say it alike. A change's own key
// stands at the end of its line instead.
func (m screen) help(look palette) string {
	l := m.s.lang
	keys := [][2]string{
		{"↑", l.text("track.huh.up", "up")},
		{"↓", l.text("track.huh.down", "down")},
		{"enter", l.text("track.huh.confirm", "confirm")},
	}

	named := make([]string, len(keys))
	for i, k := range keys {
		named[i] = look.muted.Render(k[0]+" ") + look.muted.Faint(true).Render(k[1])
	}

	return strings.Join(named, look.muted.Faint(true).Render(" • "))
}

// formHelp names the keys of the field the open form is asking about, as huh
// names them, and escape after them: huh names a field's own keys only, and
// escape is what puts the form away.
func (m screen) formHelp() string {
	cancel := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", m.s.lang.text("track.key.cancel", "cancel")))

	return m.form.Help().ShortHelpView(append(m.form.KeyBinds(), cancel))
}

// windowTitle puts the stopwatch in the terminal's title, where it can be seen
// from another tab.
func (m screen) windowTitle() string {
	if m.loaded && m.shown.clock.Running {
		return clockFace(m.elapsed()) + " · " + m.s.lang.nameOf(m.shown.projects, m.shown.clock.ProjectID)
	}

	return "go-time-recording"
}

// spread sets left and right on one line of width, cutting the left short
// rather than letting the line wrap.
func spread(width int, left, right string) string {
	if room := width - lipgloss.Width(right) - 2; lipgloss.Width(left) > room {
		left = ansi.Truncate(left, max(room, 0), "…")
	}

	return left + strings.Repeat(" ", max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)) + right
}

// clockFace writes a duration as a stopwatch shows it: 1:23:45.
func clockFace(d time.Duration) string {
	seconds := int(d.Truncate(time.Second) / time.Second)

	return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
}
