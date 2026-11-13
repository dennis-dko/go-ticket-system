//go:build browser

package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Deleting several rows at once.
//
// Only a browser can check this. The checkbox column is not written into the
// markup: it is derived, at render time, from whether a row carries a delete
// button - so whether the column appears at all, whether "select all" reaches
// every row, and whether the bar knows how many are ticked are all facts about
// running JavaScript. Reading app.js proves none of them.
//
// What would go wrong quietly: a column that appears but ticks nothing, or a bar
// that deletes one row while claiming three.
//
// The token list is the table used here because it is one anybody can fill without
// anything else existing. The mechanism is the same everywhere - people, roles,
// tokens and passkeys all get it from their own delete buttons, and an import
// preview gets nothing, because there is nothing in it to delete.
func TestSeveralRowsAreDeletedAtOnce(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	// Three tokens of the account's own, so nothing else has to exist.
	labels := []string{"first-token", "second-token", "third-token"}

	p.makeTokens(labels)

	// The column exists because the rows offer deletion, and there is one checkbox
	// per row rather than one for the table.
	if got := p.count("#table-tokens tbody input.row-pick"); got != len(labels) {
		t.Fatalf("the table has %d row checkboxes for %d deletable rows", got, len(labels))
	}

	// Nothing is ticked yet, so the bar is not in the way.
	if p.visible(".bulk-bar.shown") {
		t.Error("the bulk bar is showing with nothing selected")
	}

	// "Select all" reaches every row.
	p.run("select all", p.click(`#table-tokens thead input.pick-all`),
		chromedp.WaitVisible(".bulk-bar.shown", chromedp.ByQuery))

	if got := p.count("#table-tokens tbody input.row-pick:checked"); got != len(labels) {
		t.Fatalf(`"select all" ticked %d of %d rows`, got, len(labels))
	}

	// And the bar counts what is ticked rather than guessing.
	if count := p.text(".bulk-bar.shown .bulk-count"); !strings.Contains(count, "3") {
		t.Errorf("the bar says %q, which does not mention 3 selected rows", count)
	}

	// Deleting asks once for the batch, not once per row.
	p.run("press delete", p.click(`.bulk-bar.shown button.danger`),
		chromedp.WaitVisible(".confirm-overlay", chromedp.ByQuery))

	p.run("confirm", p.click(`.confirm-actions button.danger`))
	p.waitGone(".confirm-overlay")

	// All three gone, and the column with them: there is nothing left to delete.
	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		body := p.text("#table-tokens tbody")

		gone := true
		for _, label := range labels {
			if strings.Contains(body, label) {
				gone = false

				break
			}
		}

		if gone {
			if p.visible(".bulk-bar.shown") {
				t.Error("the bulk bar is still showing after the rows it referred to went")
			}

			return
		}

		time.Sleep(250 * time.Millisecond)
	}

	t.Errorf("the rows are still there after a bulk delete; the table says:\n%s\n\n"+
		"application log:\n%s", p.text("#table-tokens tbody"), p.app.Log())
}

// Deleting some of the rows leaves the bar telling the truth about the rest.
//
// This is the case that was wrong. The bar reads how many checkboxes are ticked out
// of the table, and it was refreshed before the new rows were in the document - so
// it counted the previous render. Delete two of three and the bar stayed up saying
// "2 selected" over a table where nothing was ticked, with a delete button that
// would have done nothing.
//
// Deleting all of them hid it by luck: with no rows left there is nothing to delete,
// so the whole column goes and the bar with it.
func TestTheBarStandsDownWhenOnlySomeRowsWereDeleted(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	labels := []string{"fourth-token", "fifth-token", "sixth-token"}

	p.makeTokens(labels)

	// Two of the three, by ticking them one at a time rather than select-all.
	p.run("tick the first two",
		p.click(`#table-tokens tbody tr:nth-child(1) input.row-pick`),
		p.click(`#table-tokens tbody tr:nth-child(2) input.row-pick`),
		chromedp.WaitVisible(".bulk-bar.shown", chromedp.ByQuery))

	if got := p.count("#table-tokens tbody input.row-pick:checked"); got != 2 {
		t.Fatalf("%d rows are ticked, want 2", got)
	}

	p.run("delete them", p.click(`.bulk-bar.shown button.danger`),
		chromedp.WaitVisible(".confirm-overlay", chromedp.ByQuery),
		p.click(`.confirm-actions button.danger`))

	p.waitGone(".confirm-overlay")

	// One row is left, so the column stays - and the bar must have stood down,
	// because nothing in the new table is ticked.
	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		if p.count("#table-tokens tbody input.row-pick") == 1 {
			if p.visible(".bulk-bar.shown") {
				t.Errorf("one row is left and nothing is ticked, but the bar is still up "+
					"saying %q", p.text(".bulk-bar.shown .bulk-count"))
			}

			if got := p.count("#table-tokens tbody input.row-pick:checked"); got != 0 {
				t.Errorf("%d checkbox(es) survived the reload, so a second delete would "+
					"act on a selection nobody made", got)
			}

			return
		}

		time.Sleep(250 * time.Millisecond)
	}

	t.Errorf("the table does not hold exactly one deletable row after deleting two of "+
		"three; it says:\n%s\n\napplication log:\n%s",
		p.text("#table-tokens tbody"), p.app.Log())
}

// A row nobody may delete gets no checkbox, and the ones beside it still line up.
//
// The cell is still there for such a row - it just has nothing in it to tick -
// because a missing cell would shift every column of that row one to the left. That
// is the kind of fault that looks like bad data rather than bad markup.
func TestARowThatCannotBeDeletedHasNoCheckboxButKeepsItsPlace(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	// A second account, so the table holds one row that may be deleted and one that
	// may not - the built-in administrator. With only the administrator there would
	// be nothing deletable at all and no column, which is a different case.
	p.run("add somebody", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible("#form-user", chromedp.ByID),
		chromedp.SendKeys(`#form-user input[name="name"]`, "Wilma", chromedp.ByQuery),
		chromedp.SendKeys(`#form-user input[name="email"]`, "wilma@example.com",
			chromedp.ByQuery),
		p.chooseOption(`#form-user select[name="role"]`, "agent"),
		chromedp.SendKeys(`#form-user input[name="password"]`, "wilma-password-1",
			chromedp.ByQuery),
		p.click(`#form-user button[type="submit"]`))

	p.waitForText("#table-users tbody", "wilma@example.com")

	// Two rows, two pick cells: the administrator's is there to keep the columns
	// lined up, which is why the cell is added even where nothing may be ticked.
	if got := p.count("#table-users tbody td.pick"); got != 2 {
		t.Errorf("%d rows have a checkbox cell, want 2; without one the row's columns "+
			"sit one to the left of the heading", got)
	}

	// And exactly one checkbox: Wilma's, not the administrator's.
	if got := p.count("#table-users tbody input.row-pick"); got != 1 {
		t.Errorf("the table offers %d checkboxes, want 1 - the built-in administrator "+
			"cannot be deleted and must not be offered for it", got)
	}

	// Every row has as many cells as the heading has columns.
	head := p.count("#table-users thead th")
	cells := p.count("#table-users tbody tr td")

	if head == 0 || cells != head*2 {
		t.Errorf("the heading has %d columns and two rows have %d cells between them, "+
			"want %d", head, cells, head*2)
	}
}

// The role dropdown says what each role is for.
//
// Somebody deciding what a colleague may do picks from this list, and "admin"
// against "agent" is a difference you can only infer from the name - the difference
// being whether that person can administer the installation. Only a browser can check
// it: the labels are built at render time from what the server sent, so reading app.js
// proves nothing about what is on screen.
func TestTheRoleDropdownExplainsEachRole(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the people screen", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible(`#form-user select[name="role"]`, chromedp.ByQuery))

	// Value and label together, because they are deliberately different things: the
	// value is the identifier the API takes, and the label is what a person reads.
	var options []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}

	p.run("read the choices", chromedp.Evaluate(
		`[...document.querySelectorAll('#form-user select[name="role"] option')]
			.map((o) => ({ value: o.value, label: o.textContent }))`, &options))

	if len(options) < 3 {
		t.Fatalf("the dropdown offers %d role(s), want the three that ship: %v",
			len(options), options)
	}

	// Each one carries more than its name. The separator is what the label builder
	// puts between the two, so its absence means the description never arrived.
	for _, option := range options {
		if !strings.Contains(option.Label, "—") {
			t.Errorf("the choice %q says only its name, so it has to be guessed at",
				option.Label)
		}

		// And no label is the identifier. Those are lowercase, hyphenated and English
		// because that is what the API takes; a person choosing what a colleague may do
		// should be reading a title in their own language, and for a while they were
		// reading the identifier.
		if strings.HasPrefix(option.Label, option.Value+" ") ||
			strings.HasPrefix(option.Label, option.Value+"—") {
			t.Errorf("the choice for %q is shown by its identifier: %q",
				option.Value, option.Label)
		}
	}

	// Found by value rather than by what it says, which is the whole point: the label
	// is translated and the value is not.
	administering := ""

	for _, option := range options {
		if option.Value == "admin" {
			administering = option.Label
		}
	}

	if administering == "" {
		t.Fatalf("the administrator's role is not offered at all: %v", options)
	}

	// German, because readyAdmin leaves the interface in whatever the browser asked
	// for and CI runs it in English - so this checks the word that is in both.
	if !strings.Contains(strings.ToLower(administering), "administ") &&
		!strings.Contains(strings.ToLower(administering), "verwalt") {
		t.Errorf("the administrator's role does not say that it administers: %q", administering)
	}
}

// The rights of a system role are shown as unavailable, not offered and then refused.
//
// The name of a system role has been read-only in this form all along; its permissions
// were left clickable, and saving them came back as a refusal. A control that looks
// usable and answers "no" is worse than one that says so first - and this is the screen
// where somebody is deciding what an account may do, so being told afterwards is the
// worst moment to find out.
//
// Only a browser can check it: the checkboxes are built at render time from what the
// server sent, and whether they are disabled is not a fact about the source.
func TestASystemRolesRightsCannotBeTicked(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the roles screen", p.click(`.tab[data-view="roles"]`),
		chromedp.WaitVisible("#table-roles", chromedp.ByID))

	// The built-in administrator's own role, which is the system one.
	p.waitForText("#table-roles tbody", "admin")

	p.run("edit it", chromedp.Click(
		`#table-roles tbody tr:nth-child(1) button.link`, chromedp.ByQuery),
		chromedp.WaitVisible("#permission-list input", chromedp.ByQuery))

	// Every box, not merely the first: a loop that disabled one would look right in a
	// screenshot.
	offered := p.count(`#permission-list input[name="permissions"]`)
	locked := p.count(`#permission-list input[name="permissions"]:disabled`)

	if offered == 0 {
		t.Fatal("the form offers no permissions at all, so this proves nothing")
	}

	if locked != offered {
		t.Errorf("%d of %d rights can still be ticked on a system role", offered-locked, offered)
	}

	// And the reason is on screen, next to them.
	if !p.visible("#role-fixed-note") {
		t.Error("nothing says why the rights cannot be changed")
	}

	// The name too, which was already the case - checked here so the two stay together.
	//
	// Asked as a property. The first version asked for the readonly attribute and
	// failed against an interface that was doing the right thing - a boolean attribute
	// present in the markup has the empty string as its value, so comparing it against
	// "" answers the same either way.
	if !p.locked(`#form-role input[name="name"]`) {
		t.Error("the name of a system role can be edited")
	}
}

// The account table gets the same column, and the one row nobody may delete
// gets a cell with nothing in it.
//
// The mechanism is derived rather than written down - a row that carries a
// delete button gets a checkbox, and a table with any such row grows the column
// - so "it works for one table" says nothing about the others. This is the table
// where that matters most: it is the one with a permanently undeletable row in
// the middle of it, and a select-all that ticked the built-in administrator
// would offer a deletion the server refuses.
func TestTheAccountTableOffersTheSameSelection(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	for _, who := range []string{"eins@example.com", "zwei@example.com"} {
		p.createOrdinaryAccount(t, who, "another-password-1")
	}

	// Reloaded, because the accounts above were created through the API rather
	// than through the form. The table is filled when the interface loads and
	// refreshed by what the form does afterwards, so switching to the tab shows
	// what was there at sign-in - which is neither of them.
	p.run("reload", chromedp.Reload(), chromedp.WaitVisible("#tabs", chromedp.ByID))
	p.settleWelcome()

	p.run("open the accounts", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible("#table-users", chromedp.ByID))

	p.waitForText("#table-users tbody", "zwei@example.com")

	// The column is there at all, which is the question the screenshot of a table
	// without it raises.
	if p.count("#table-users thead th.pick") != 1 {
		t.Fatal("the account table has no selection column, so several accounts " +
			"cannot be deleted at once")
	}

	// Three rows, two of them deletable: the built-in administrator is not, and
	// it gets a cell so the columns still line up.
	if got := p.count("#table-users tbody td.pick"); got != 3 {
		t.Errorf("%d rows carry a selection cell, want 3 - one per account", got)
	}

	if got := p.count("#table-users tbody input.row-pick"); got != 2 {
		t.Errorf("%d accounts can be ticked, want 2 - the built-in administrator "+
			"must not be one of them", got)
	}

	// Select all reaches exactly those two, rather than the row it cannot delete.
	p.run("select all", p.click(`#table-users thead input.pick-all`),
		chromedp.WaitVisible(".bulk-bar.shown", chromedp.ByQuery))

	if got := p.count("#table-users tbody input.row-pick:checked"); got != 2 {
		t.Errorf("select all ticked %d rows, want 2", got)
	}

	if count := p.text(".bulk-bar.shown .bulk-count"); !strings.Contains(count, "2") {
		t.Errorf("the bar says %q, which does not name the two rows that are ticked", count)
	}
}

// makeTokens creates one personal token for each label through the token form,
// and waits until each is listed.
func (p *page) makeTokens(labels []string) {
	p.t.Helper()

	p.showView("settings")
	p.waitShown("#form-token")

	for _, label := range labels {
		p.run("make the token "+label,
			chromedp.SetValue(`#form-token input[name="name"]`, label, chromedp.ByQuery),
			p.click(`#form-token button[type="submit"]`))

		p.waitForText("#table-tokens tbody", label)
	}
}
