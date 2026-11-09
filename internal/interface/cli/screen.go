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

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
)

// refreshEvery is how often the screen reads the day again while it is open.
// The stopwatch is the one the browser shows, so it may have been stopped
// there; half a minute is soon enough to notice that, and rare enough to cost
// the installation nothing.
const refreshEvery = 30 * time.Second

// screen is the client on a terminal: the web interface's tabs, under them the
// stopwatch running live and the entries, a menu of the changes it can take,
// and a form for each of them.
//
// It makes those changes through the same session as the commands, so it can
// do nothing they cannot, and it says what it did in their words. It keeps the
// context it was opened with because Bubble Tea hands Update none, and every
// request the screen sends has to end when the program does.
type screen struct {
	ctx  context.Context
	s    *session
	host string

	// width and height are the window's: how wide the frame is drawn, and how
	// many of the entries fit in it.
	width, height int

	// view is the tab moved to, by the name the page gives it; see currentTab.
	// sub is the card whose menu is open under it, empty for the tab's own.
	view, sub string

	// card is the entries card, and top the first of its entries on screen where
	// the window holds fewer than were read.
	card entryCard
	top  int

	// week is the week grid as it was last read, and weekShown the Monday of the
	// week it was moved to - empty for this week, which it then follows.
	week      weekCard
	weekShown string

	// cell is the cell of the week grid the cursor is on, and added the rows added
	// to a week's grid by hand.
	cell  weekCell
	added []addedRow

	// cal is the calendar's tab: the month and the cards beside it.
	cal calendarState

	// picking is the change an entry is being picked for in the card, and row the
	// entry the cursor is on, by its place among those read. editing is the entry
	// picked, while its form is open and its change on its way.
	picking string
	row     int
	editing entry

	// ticked are the entries ticked for a change made to several, by id.
	ticked []uint

	// templates are the account's, read with the day, and chosen the one picked
	// to be deleted.
	templates []template
	chosen    template

	// plan is a file checked for an import and not written yet.
	plan *importPlan

	// past is an entry's history, shown until it is closed.
	past *history

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

// loaded is the day as the installation answered, the entries card, what the
// tab on screen shows besides, the templates, what it calls itself and says, the
// notices, and the moment it was read. carded says whether the account may read
// its entries, templated whether its templates were read, and branded, noticed
// and placed whether the rest could be read at all.
type loaded struct {
	shown     snapshot
	card      entryCard
	tab       tabRead
	templates []template
	branding  branding
	told      notices
	places    holidayPlaces
	at        time.Time
	err       error

	carded, templated, branded, noticed, placed bool
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

	return screen{ctx: ctx, s: s, host: host, view: viewTimesheets, reading: true, askedAt: time.Now()}
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

// load reads the day again, the entries card as it is narrowed and as far as it
// has been read, what the tab on screen shows besides, what the installation
// calls itself and says, and the notices. A name, a banner or notices that could
// not be asked for leave the ones shown: nothing else depends on them, and an
// installation from before they were answered says nothing about them.
func (m screen) load() tea.Cmd {
	ctx, s, filter, shown := m.ctx, m.s, m.card.filter, len(m.card.items)
	view, monday, first := m.currentTab().view, m.weekMonday(), m.calendarMonth()

	return func() tea.Msg {
		day, err := s.day(ctx)
		if err != nil {
			return loaded{at: time.Now(), err: err}
		}

		var (
			card entryCard
			tab  tabRead
		)

		carded := slices.Contains(day.permissions, model.PermTimesheetReadOwn)
		if carded {
			if card, err = s.readCard(ctx, filter, shown); err != nil {
				return loaded{at: time.Now(), err: err}
			}
		}

		if carded {
			if tab, err = s.readTab(ctx, view, monday, first); err != nil {
				return loaded{at: time.Now(), err: err}
			}
		}

		// The templates of an account that may book, as the page reads them; ones
		// that could not be read leave those shown, like the name and the notices.
		var (
			kept      []template
			templated bool
		)

		if slices.Contains(day.permissions, model.PermTimesheetWriteOwn) {
			var keptErr error

			kept, keptErr = s.templates(ctx)
			templated = keptErr == nil
		}

		named, nameErr := s.branding(ctx)
		told, toldErr := s.notices(ctx)
		places, placesErr := s.holidayPlaces(ctx)

		return loaded{shown: day, card: card, tab: tab, templates: kept, branding: named, told: told, places: places,
			at: time.Now(), carded: carded, templated: templated, branded: nameErr == nil,
			noticed: toldErr == nil, placed: placesErr == nil}
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
		m.width, m.height = msg.Width, msg.Height
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
	case morePage:
		return m.addPage(msg), nil
	case weekRead:
		return m.tookWeek(msg), nil
	case monthRead:
		return m.tookMonth(msg), nil
	case history:
		return m.showHistory(msg), nil
	case checked:
		return m.took(msg)
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

	// The card as it was asked for: one read under a filter changed since belongs
	// to a list that is gone, and the read the change started is on its way.
	switch {
	case !msg.carded:
		m.card = entryCard{filter: m.card.filter}
	case msg.card.filter == m.card.filter:
		m.card = msg.card
	}

	m = m.tookTab(msg.tab)

	if msg.templated {
		m.templates = msg.templates
	}

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

// View draws the screen, with as many of the entries as the window has room for.
func (m screen) View() tea.View {
	v := tea.NewView(m.frame(m.listRoom()))
	v.AltScreen = true
	v.WindowTitle = m.windowTitle()

	return v
}

// listRoom is how many of the entries the window has room for: its height less
// everything else on the screen, and never fewer than a list needs to be read.
// A window that has not said how high it is has room for all of them.
func (m screen) listRoom() int {
	if m.height == 0 {
		return len(m.card.items)
	}

	return max(m.height-lipgloss.Height(m.frame(0)), listFewest)
}

// frame draws the screen in a frame, with rows of the entries: the
// installation's name, who and which build, the page's tabs, where on the screen
// somebody is, and under it either what the tab shows and its menu, or the form
// that is open.
func (m screen) frame(rows int) string {
	look := paletteFor(m.darkness())
	width := m.inner()
	rule := look.muted.Render(strings.Repeat("─", width))

	// What belongs together is set without a line between: the name and who is
	// signed in, the breadcrumb between its rules, and a question and the menu
	// that answers it. A blank line between those cost five lines the entries
	// need, once the screen carried the page's tabs and cards.
	parts := []string{
		lipgloss.PlaceHorizontal(width, lipgloss.Center, look.title.Render(m.heading())) + "\n" + m.subtitle(look, width),
	}

	// Wrapped rather than cut short, as everything that says something is.
	wrapped := lipgloss.NewStyle().Width(width)

	if m.banner != "" {
		parts = append(parts, wrapped.Render(look.ask.Render(m.banner)))
	}

	if m.loaded {
		parts = append(parts, m.tabBar(look, width))
	}

	parts = append(parts, rule+"\n"+m.breadcrumb(look)+"\n"+rule)

	switch {
	case m.form != nil:
		parts = append(parts, m.form.View(), m.formHelp())
	case m.past != nil:
		parts = append(parts, m.historyView(look, width))
	default:
		for _, line := range m.noticeLines() {
			parts = append(parts, wrapped.Render(look.ask.Render("! "+line)))
		}

		parts = append(parts, m.tabBody(look, width, rows)...)

		// While an entry is picked, the question is which, and the menu waits.
		if m.picking != "" {
			parts = append(parts, rule+"\n"+wrapped.Render(look.ask.Render(m.pickPrompt())))
		} else {
			parts = append(parts, rule+"\n"+look.ask.Render(m.menuPrompt())+"\n"+m.menu(look, width))
		}
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

	return lipgloss.NewStyle().Margin(1, 2).Render(frame.Render(strings.Join(parts, "\n\n")))
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

// breadcrumb says where on the screen somebody is: the tab, the card whose menu
// is open, the change an entry is being picked for or whose answer is shown,
// and while a form is open the change it asks about and the step it is at, the
// last of them in the accent. A step called what its change is called is said
// once.
func (m screen) breadcrumb(look palette) string {
	steps := []string{m.tabName(m.currentTab())}

	if m.sub != "" {
		steps = append(steps, m.subName())
	}

	switch {
	case m.picking != "":
		steps = append(steps, m.calledFor(m.picking))
	case m.past != nil:
		steps = append(steps, m.calledFor("history"))
	case m.form != nil:
		change := m.changeName()
		steps = append(steps, change)

		if step := m.stepName(); step != "" && step != change {
			steps = append(steps, step)
		}
	}

	for i := range steps[:len(steps)-1] {
		steps[i] = look.muted.Render(steps[i])
	}

	steps[len(steps)-1] = look.title.Render(steps[len(steps)-1])

	return strings.Join(steps, look.muted.Render(" › "))
}

// changeName is what the menu calls the change the open form asks about, and
// for a setting what the setting is called.
func (m screen) changeName() string {
	switch m.doing {
	case "settings":
		return m.settingName(m.asked.setting)
	case "bookTemplate":
		return m.calledFor("book")
	}

	return m.calledFor(m.doing)
}

// calledFor is what the menu calls a change, in whichever of its menus offers
// it.
func (m screen) calledFor(doing string) string {
	running := m
	running.shown.clock.Running = true

	for _, menu := range [][]choice{m.choicesFor(doing), running.clockChoices(), m.entryChoices(), m.templateChoices(),
		m.workbookChoices(), m.weekChoices(), m.calendarChoices()} {
		for _, c := range menu {
			if c.doing == doing {
				return c.called
			}
		}
	}

	return doing
}

// choicesFor is the stopwatch's menu as it stood when doing was chosen from it:
// a switch and a stop were chosen from a running stopwatch's.
func (m screen) choicesFor(doing string) []choice {
	shown := m
	shown.shown.clock.Running = doing == "switch" || doing == "stop"

	return shown.timesheetChoices()
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
	case "from":
		return l.text("field.from", "From")
	case "to":
		return l.text("field.to", "To")
	case "search":
		return l.text("filter.search", "Search in descriptions")
	case "scope":
		return l.text("field.project", "Project")
	case "clockDay":
		return l.text("field.date", "Date")
	case "clockTime":
		return l.text("timer.time", "Time")
	case "template":
		return l.text("tpl.one", "Template")
	case "file":
		return l.text("track.wb.file", "File")
	case "row":
		return l.text("field.project", "Project")
	}

	return m.settingStep(m.form.GetFocusedField().GetKey())
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
