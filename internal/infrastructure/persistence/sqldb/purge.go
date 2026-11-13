package sqldb

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// PurgeUser removes an account and everything that is the account's own, and
// takes its name off what is not.
//
// The two are different on purpose. What somebody signs in with, what they asked
// to be told about and which browsers they are pushed to are theirs, and go with
// them. A ticket is the organisation's record: the one an agent owned stays, and
// is left without an owner and free for somebody else to take, and what the
// agent wrote in it stays too - signed by an id that names nobody once the
// account is gone. A customer's contact stays as well, without the account they
// reached the portal with.
//
// This is irreversible. It is used by directory synchronisation, where an
// account that disappeared upstream must not linger here, and by an
// administrator deleting an account deliberately.
//
// # Why it is a transaction
//
// The final DELETE is the one that can be refused: a database that enforces the
// foreign keys rejects it while anything still points at the account. Without a
// transaction everything before it has already happened by then, so the account
// remains with its sessions, tokens and tickets gone from it, and repeating the
// purge cannot bring them back. If the reason is permanent - a table this list
// forgot - the state is permanent too.
//
// Inside a transaction the same failure leaves the account as it was, and the
// error says what stopped it.
func (r *UserRepository) PurgeUser(ctx context.Context, userID uint) error {
	return r.withTx(ctx, func(tx base) error {
		// A ticket somebody else can take is one that is not locked, so the lock
		// goes with the owner; one statement for the three columns, because a
		// locked ticket with no owner is a ticket nobody may work on.
		if _, err := tx.exec(ctx,
			"UPDATE tickets SET owner_id = NULL, locked = ?, locked_at = NULL WHERE owner_id = ?",
			false, userID); err != nil {
			return apperror.Internal(fmt.Errorf("releasing the tickets of user %d: %w", userID, err))
		}

		// Every table with a foreign key to users has to appear here. One that is
		// missing does not fail quietly: on PostgreSQL it makes the final delete
		// impossible, and on SQLite and MySQL - neither of which enforces the keys
		// this schema declares - it leaves rows pointing at an account that no
		// longer exists.
		steps := []struct {
			what  string
			query string
		}{
			{"responsibility for tickets", "UPDATE tickets SET responsible_id = NULL WHERE responsible_id = ?"},
			{"link to a contact", "UPDATE contacts SET user_id = NULL WHERE user_id = ?"},
			{"watched tickets", "DELETE FROM ticket_watchers WHERE user_id = ?"},
			{"notifications", "DELETE FROM notifications WHERE user_id = ?"},
			{"queues to be told about", "DELETE FROM user_queues WHERE user_id = ?"},
			{"api tokens", "DELETE FROM api_tokens WHERE user_id = ?"},
			{"passkeys", "DELETE FROM passkeys WHERE user_id = ?"},
			{"sessions", "DELETE FROM sessions WHERE user_id = ?"},
			{"push subscriptions", "DELETE FROM push_subscriptions WHERE user_id = ?"},
			{"openid identities", "DELETE FROM openid_identities WHERE user_id = ?"},
		}

		for _, step := range steps {
			if _, err := tx.exec(ctx, step.query, userID); err != nil {
				return apperror.Internal(fmt.Errorf("removing the %s of user %d: %w", step.what, userID, err))
			}
		}

		affected, err := tx.exec(ctx, "DELETE FROM users WHERE id = ?", userID)
		if err != nil {
			return apperror.Internal(err)
		}

		if affected == 0 {
			return apperror.NotFound("user", strconv.FormatUint(uint64(userID), 10))
		}

		return nil
	})
}

// CountUnfinishedOwnedBy is how many tickets an account owns that somebody still
// has to answer: the ones PurgeUser would leave without an owner and that
// anybody would miss. A finished ticket loses its owner's name the same way,
// and is not counted, because nobody is waiting on it.
func (r *UserRepository) CountUnfinishedOwnedBy(ctx context.Context, userID uint) (int, error) {
	finished := model.FinishedStates()

	args := make([]any, 0, len(finished)+1)
	args = append(args, userID)

	for _, state := range finished {
		args = append(args, string(state))
	}

	var count int

	err := r.db.QueryRowContext(ctx, r.rebind(
		"SELECT COUNT(*) FROM tickets WHERE owner_id = ? AND state NOT IN "+marks(len(finished))),
		args...).Scan(&count)
	if err != nil {
		return 0, apperror.Internal(err)
	}

	return count, nil
}
