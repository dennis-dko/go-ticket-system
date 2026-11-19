//go:build browser

package browser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
)

// The account list offers to remove a second factor only where there is one,
// and removing it takes it away.
//
// Somebody who lost their phone cannot turn their own second factor off, so the
// administrator does it from their row. A button on every row would offer a
// change the server refuses for most of them - nothing to remove - and a button
// that asks for a refusal teaches somebody the screen is broken. So it is on the
// row of an account that holds one, and nowhere else: not on an account without
// one, not on the administrator's own, and gone once it has been pressed.
func TestTheAccountListOffersToRemoveASecondFactorOnlyWhereThereIsOne(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()
	p.createOrdinaryAccount(t, "lost@example.com", "lost-phone-password-1")
	p.createOrdinaryAccount(t, "kept@example.com", "kept-phone-password-1")

	var id uint

	p.awaitJSON(`(async () => {
		const accounts = await api('/users');
		return accounts.items.find((u) => u.email === 'lost@example.com').id;
	})()`, &id)

	// A passkey on the phone that was lost, put in through the application's own
	// repository: registering one takes an authenticator, and what is asked here
	// is only who the screen offers the removal to.
	keys := sqldb.NewPasskeyRepository(p.app.DB(t), p.app.Dialect())
	if _, err := keys.Save(context.Background(), &model.Passkey{
		UserID: id, Name: "phone", CredentialID: []byte("the-lost-phone"),
		PublicKey: []byte("its-key"), CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("give the account a passkey: %v", err)
	}

	p.run("reload", chromedp.Reload(), chromedp.WaitVisible("#tabs", chromedp.ByID))
	p.settleWelcome()

	p.run("open the accounts", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible("#table-users", chromedp.ByID))

	const offer = `#table-users tbody button[data-action="remove-second-factor"]`

	p.waitForNode(offer)

	if got := p.count(offer); got != 1 {
		t.Fatalf("the account list offers to remove a second factor on %d rows, want only the one "+
			"account that holds one", got)
	}

	var row string

	p.run("read the row it is on", chromedp.Evaluate(
		`document.querySelector('`+offer+`').closest('tr').textContent`, &row))

	if !strings.Contains(row, "lost@example.com") {
		t.Fatalf("the removal is offered on the wrong row: %q", row)
	}

	p.run("remove it", p.click(offer),
		chromedp.WaitVisible(".confirm-card button.confirm-proceed", chromedp.ByQuery),
		p.click(".confirm-card button.confirm-proceed"))

	p.waitForText("#toast", "econd factor removed")
	p.waitGone(offer)

	if n := p.app.Rows(t, "passkeys", "user_id = ?", id); n != 0 {
		t.Errorf("the account still holds %d passkeys", n)
	}
}

// The administration log names the server's command line for a recovery.
//
// The recover command changes an account with no account of its own - nobody
// can sign in, which is what it is for - so the log records it by nobody. Read
// as an account, that was "#0" in the column saying who: a number that names no
// one and looks like a fault. The column says where it came from instead, in
// either language, and the sentence names the account it was done to.
func TestTheLogNamesTheCommandLineForARecovery(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the settings", p.click(`.tab[data-view="admin"]`))
	p.showTabOf("#changes-card")

	for _, want := range []struct{ language, who, what string }{
		{"en", "The command line on the server", "Recovered Administrator: a new password, no second factor"},
		{"de", "Die Kommandozeile auf dem Server", "Administrator wiederhergestellt: neues Passwort, kein zweiter Faktor"},
	} {
		if want.language != "en" {
			p.chooseLanguage(want.language)
		}

		var cells []string

		p.run("draw a recovery in the log", chromedp.Evaluate(`(() => {
			adminChanges = {
				entries: [{
					id: 99, at: new Date().toISOString(), action: 'user.recovered',
					actor: { id: 0, name: '' }, target: { id: 1, name: 'Administrator' },
				}],
				more: false,
			};
			redrawable('changes', drawChanges);

			const row = document.querySelector('#table-changes tbody tr');
			return [...row.cells].map((cell) => cell.textContent);
		})()`, &cells))

		if len(cells) < 3 || cells[1] != want.who || cells[2] != want.what {
			t.Errorf("%s: the log reads %q, want %q by %q", want.language, cells, want.what, want.who)
		}
	}
}
