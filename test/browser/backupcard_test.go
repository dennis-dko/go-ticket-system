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
