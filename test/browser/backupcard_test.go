//go:build browser

package browser

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// The backup card makes a backup, lists it, and keeps a plan.
//
// Pressed, "Back up now" leaves a row in the card's table for the backup it
// made - the download beside it is the API's to prove, which the integration
// suite does - and a plan saved on the card is the plan the card shows again
// once it has been drawn from what the server kept.
func TestTheBackupCardMakesAndKeepsABackup(t *testing.T) {
	t.Parallel()

	p := openWith(t, "BACKUP_PASSPHRASE=the installation's own passphrase")
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#backup-card")

	if p.visible("#backup-off") || !p.visible("#backup-now") {
		t.Fatal("with a passphrase on the server, the card says none is set or offers no button")
	}

	p.run("make a backup", p.click("#backup-now"))
	p.waitForText("#toast", "ackup made")
	p.waitForNode(`#table-backups tbody button[data-action="download"]`)

	var made string

	p.run("read the row", chromedp.Evaluate(
		`document.querySelector('#table-backups tbody tr').textContent`, &made))

	if strings.Contains(made, "No backup") {
		t.Fatalf("after a backup the table reads %q", made)
	}

	p.run("plan a backup every night", chromedp.SetValue(`#form-backup-plan input[name="schedule"]`,
		"0 3 * * *", chromedp.ByQuery),
		chromedp.SetValue(`#form-backup-plan input[name="keep"]`, "5", chromedp.ByQuery),
		p.click(`#form-backup-plan button[type="submit"]`))
	p.waitForText("#toast", "aved")

	var plan struct {
		Schedule string `json:"schedule"`
		Keep     string `json:"keep"`
	}

	p.evalJSON(`JSON.stringify({
		schedule: document.querySelector('#form-backup-plan input[name="schedule"]').value,
		keep: document.querySelector('#form-backup-plan input[name="keep"]').value,
	})`, &plan)

	if plan.Schedule != "0 3 * * *" || plan.Keep != "5" {
		t.Errorf("after saving, the card shows the plan %+v, want 0 3 * * * keeping 5", plan)
	}
}

// Without BACKUP_PASSPHRASE the card says what to set, how long it has to be,
// and offers no button the server would refuse.
func TestTheBackupCardSaysWhatToSetWithoutAPassphrase(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#backup-card")

	if p.visible("#backup-now") {
		t.Error("without a passphrase on the server the card offers to make a backup the server refuses")
	}

	said := p.text("#backup-off")
	if !p.visible("#backup-off") || !strings.Contains(said, "BACKUP_PASSPHRASE") || !strings.Contains(said, "12") {
		t.Errorf("the card says %q, want what to set and that it takes 12 characters", said)
	}
}

// The plan stands a line apart from the button above it.
//
// "Back up now" makes a backup at once; the form under it only stores when the
// next ones are made. With nothing between them the schedule's label read as
// the button's caption, "Zeitplan" directly under "Jetzt sichern". .row-apart
// is the stylesheet's rule for this: a line's worth of air above a control that
// is a different kind of thing from the one above it.
//
// Measured as geometry rather than asserted as a class, as the rights notice's
// case is, so it says what a reader would see.
func TestTheBackupPlanStandsApartFromTheButtonAboveIt(t *testing.T) {
	t.Parallel()

	p := openWith(t, "BACKUP_PASSPHRASE=the installation's own passphrase")
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#backup-card")
	p.waitShown("#backup-now")

	var gap int

	p.run("measure the air under the button", chromedp.Evaluate(`(() => {
		const button = document.querySelector('#backup-now');
		const label = document.querySelector('#form-backup-plan label');

		return Math.round(
			label.getBoundingClientRect().top - button.getBoundingClientRect().bottom);
	})()`, &gap))

	p.run("show the whole card", chromedp.ScrollIntoView("#backup-card"))
	p.saveShot(t, "backupcard.png")

	t.Logf("the schedule's label sits %dpx below the button", gap)

	// A line, near enough: the rule is 18px, and this asks for most of one so the
	// case is about there being air rather than about the exact figure.
	const aLine = 12

	if gap < aLine {
		t.Errorf("the schedule's label sits %dpx under \"Back up now\", which reads as "+
			"the button's caption. .row-apart already exists for this: a line's worth "+
			"of air above a control that is a different kind of thing from the one "+
			"above it", gap)
	}
}

// The card's explanations are drawn as explanations, not as warnings.
//
// .minus is the stylesheet's warning - the danger colour, in bold - and the
// folder line and the schedule's hint carried it while saying nothing that is
// wrong: where the backups are kept, and how the five fields are read. Red on a
// card about keeping the data safe says something is the matter with it. The
// directory card's schedule hint, the same kind of sentence, is plain .muted.
func TestTheBackupCardsExplanationsAreNotWarnings(t *testing.T) {
	t.Parallel()

	p := openWith(t, "BACKUP_PASSPHRASE=the installation's own passphrase")
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#backup-card")
	p.waitShown("#backup-now")

	var looks map[string]string

	p.run("read how each sentence is drawn", chromedp.Evaluate(`(() => {
		const look = (selector) => {
			const style = getComputedStyle(document.querySelector(selector));
			return style.color + ' ' + style.fontWeight;
		};

		return {
			card: look('#backup-card > p[data-i18n="backup.hint"]'),
			folder: look('#backup-folder'),
			schedule: look('#form-backup-plan p[data-i18n="backup.scheduleHint"]'),
		};
	})()`, &looks))

	for _, line := range []string{"folder", "schedule"} {
		if looks[line] != looks["card"] {
			t.Errorf("the %s line is drawn %q where the card's own explanation is %q; "+
				"it says nothing that is wrong, and .minus is the warning", line, looks[line], looks["card"])
		}
	}
}
