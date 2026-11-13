//go:build browser

package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/go-ticket-system/internal/support/spreadsheet"
)

// An import checked by one account leaves nothing behind for the next.
//
// The import cards are not forms, so the sign-out's form reset never reached
// them: the file stayed chosen, the verdict stayed written and the Import button
// stayed offered - a clean verdict brings it with it. The checked rows went too
// little further: the sign-out redraws for the language, which drew a kept
// verdict straight back, and the tables' own cards were never emptied at all. So
// whoever signed in next found the last person's rows and a button that would
// write them as their own.
func TestASignOutTakesAnImportsFileAndVerdictWithIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	people, err := spreadsheet.WriteUsers("en", []spreadsheet.UserRow{
		{Name: "A name of somebody else's", Email: "else@example.com", Role: "agent"},
	})
	if err != nil {
		t.Fatalf("building the people: %v", err)
	}

	roles, err := spreadsheet.WriteRoles("en", []string{"tickets:read"}, []spreadsheet.RoleRow{
		{Name: "elsewhere", Description: "A role of somebody else's", Granted: []string{"tickets:read"}},
	})
	if err != nil {
		t.Fatalf("building the roles: %v", err)
	}

	peoplePath := filepath.Join(dir, "people.xlsx")
	rolesPath := filepath.Join(dir, "roles.xlsx")

	for path, book := range map[string][]byte{peoplePath: people, rolesPath: roles} {
		if err := os.WriteFile(path, book, 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	p := open(t)
	p.readyAdmin()

	const (
		peopleCard = `#view-users .card:has(input[type="file"])`
		rolesCard  = `#view-roles .card:has(input[type="file"])`
	)

	p.run("check a file of people", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible(peopleCard, chromedp.ByQuery),
		chromedp.SetUploadFiles(peopleCard+` input[type="file"]`, []string{peoplePath}, chromedp.ByQuery),
		p.click(peopleCard+` button[data-i18n="wb.preview"]`))
	p.waitForText(peopleCard+" tbody", "A name of somebody else's")

	p.run("check a file of roles", p.click(`.tab[data-view="roles"]`),
		chromedp.WaitVisible(rolesCard, chromedp.ByQuery),
		chromedp.SetUploadFiles(rolesCard+` input[type="file"]`, []string{rolesPath}, chromedp.ByQuery),
		p.click(rolesCard+` button[data-i18n="wb.preview"]`))
	p.waitForText(rolesCard+" tbody", "A role of somebody else's")

	p.run("sign out", p.click("#logout"), chromedp.WaitVisible("#form-login", chromedp.ByID))

	var left struct {
		Rows, Files int
		Offered     bool
		Said        string
	}

	p.run("read what the next account would find", chromedp.Evaluate(`(() => {
		const cards = ['`+peopleCard+`', '`+rolesCard+`'].map((s) => document.querySelector(s));

		return {
			rows: cards.filter((c) => /somebody else's/.test(c.querySelector('tbody').textContent)).length,
			files: cards.reduce((n, c) => n + c.querySelector('input[type="file"]').files.length, 0),
			offered: cards.some((c) => !c.querySelector('button[data-i18n="wb.import"]').hidden),
			said: cards.map((c) => c.querySelector('p.muted:not([data-i18n])')?.textContent ?? '').join(''),
		};
	})()`, &left))

	if left.Rows > 0 {
		t.Errorf("%d import card(s) still show the rows the last account checked", left.Rows)
	}

	if left.Files > 0 {
		t.Errorf("%d file(s) are still chosen for whoever signs in next", left.Files)
	}

	if left.Offered {
		t.Error("an Import button is still offered for the last account's file")
	}

	if left.Said != "" {
		t.Errorf("the verdict on the last account's file is still written: %q", left.Said)
	}
}

// A form left correcting a record goes back to creating one when its account
// signs out.
//
// The sign-out resets every form, and a reset leaves a hidden field's value
// where it was - which is why each form's own reset clears its id by hand. Those
// resets were not called on the way out, so a form somebody had been correcting
// kept the record's id under its "edit" heading. Whoever signed in next and
// filled it in to make something new changed the record the last person had
// open: an administrator adding an account overwrote the one their predecessor
// was correcting.
func TestASignOutPutsEveryFormBackToCreating(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	// The administrator's own row offers no correction, so somebody else's first.
	p.run("add somebody", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible("#form-user", chromedp.ByID),
		chromedp.SendKeys(`#form-user input[name="name"]`, "Wilma", chromedp.ByQuery),
		chromedp.SendKeys(`#form-user input[name="email"]`, "wilma@example.com", chromedp.ByQuery),
		p.chooseOption(`#form-user select[name="role"]`, "agent"),
		chromedp.SendKeys(`#form-user input[name="password"]`, "wilma-password-1", chromedp.ByQuery),
		p.click(`#form-user button[type="submit"]`))
	p.waitForText("#table-users tbody", "wilma@example.com")

	p.run("start correcting the account", p.click(`#table-users tbody button[data-action="edit"]`))

	// A shipped role is opened to be looked at, through the same form and with
	// its id in the same field.
	p.run("open a role", p.click(`.tab[data-view="roles"]`),
		chromedp.WaitVisible(`#table-roles tbody td.actions button`, chromedp.ByQuery),
		p.click(`#table-roles tbody td.actions button`))

	// Read from the forms rather than the screen: only one of the two tabs is
	// showing.
	var correcting bool

	p.run("confirm both forms are correcting", chromedp.Evaluate(`[['user', 'Add user'], ['role', 'Create role']]
		.every(([kind, creating]) => document.querySelector('#form-' + kind).elements.id.value
			&& document.querySelector('#' + kind + '-form-title').textContent.trim() !== creating)`, &correcting))

	if !correcting {
		t.Fatal("this case starts from both forms correcting a record, and they are not")
	}

	p.run("sign out", p.click("#logout"), chromedp.WaitVisible("#form-login", chromedp.ByID))

	var left []string

	p.run("read what the next account would find", chromedp.Evaluate(`(() => {
		const left = [];

		for (const [kind, creating] of [['user', 'Add user'], ['role', 'Create role']]) {
			const form = document.querySelector('#form-' + kind);
			const heading = document.querySelector('#' + kind + '-form-title').textContent.trim();

			if (form.elements.id.value) left.push('the ' + kind + ' form still holds the id ' + form.elements.id.value);
			if (heading !== creating) left.push('the ' + kind + ' form is still headed ' + JSON.stringify(heading));
		}

		return left;
	})()`, &left))

	for _, what := range left {
		t.Errorf("after the sign-out %s", what)
	}
}

// The greeting an account was shown goes with it as well.
//
// The greeting names whoever it greeted, and is drawn again only when somebody
// arrives on it - so whoever signed in next at the same desk could open it on the
// last person's name. The sign-out's own comment counts what is left under a tab
// that is merely hidden as handed to the next person.
func TestASignOutTakesTheGreetingWithIt(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.run("be greeted", p.click("#app-title"))
	p.waitForFilled("#welcome-title")

	p.run("sign out", p.click("#logout"), chromedp.WaitVisible("#form-login", chromedp.ByID))

	if said := p.text("#welcome-title"); said != "" {
		t.Errorf("after the sign-out the greeting still says %q", said)
	}
}

// A sign-out takes the notices in the corner with it.
//
// A notice lingers for up to twenty seconds, and the stack sits above the sign-in
// screen on purpose - so whoever signed in next at the same desk read the last
// account's "Password changed", or a failure with that account's detail folded
// under it. Found by walking the interface: the administrator's password notice
// was still on screen after signing in as somebody else.
func TestASignOutTakesItsNoticesWithIt(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("raise a notice", chromedp.Evaluate(
		`toast('Something only this account was told')`, nil))
	p.waitShown("#toast .toast-note")

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	if left := p.text("#toast"); left != "" {
		t.Errorf("the sign-in screen still carries the last session's notice: %q", left)
	}
}
