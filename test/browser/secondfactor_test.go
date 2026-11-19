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
