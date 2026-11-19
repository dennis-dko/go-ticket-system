package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/support/apperror"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// RecoveredAction is what the administration log calls the command line giving
// the built-in administrator a new password. It is the one change no request
// makes, so the log's middleware never sees it and this service writes the
// entry itself - with no account as the one who made it. Under user., so the
// log names the account it was made to.
const RecoveredAction = "user.recovered"

// ErrRecoveryNotLogged says a recovery worked and the administration log could
// not record it. The password is still the one returned with it, and nobody
// else knows it: withholding it for want of a log line would lock out the very
// person the command is for.
var ErrRecoveryNotLogged = errors.New("the recovery worked, and the administration log could not record it")

// RecoveryService is the way back into an account nobody can sign into.
//
// Two ways, for two people. An administrator removes the second factor of
// somebody who lost the phone it was on - an authenticator code cannot be
// typed from a phone that is gone, and only its owner could turn it off, with
// a code from that phone. And the command line gives the built-in administrator
// a new password when the old one is lost and nobody is left who could set one:
// whoever can run a command on the server can already read the database, so it
// opens nothing that was closed.
type RecoveryService struct {
	users    repository.UserRepository
	passkeys repository.PasskeyRepository
	sessions repository.SessionRepository
	log      repository.AdminEventRepository
}

// NewRecoveryService wires the repositories a recovery touches.
func NewRecoveryService(
	users repository.UserRepository,
	passkeys repository.PasskeyRepository,
	sessions repository.SessionRepository,
	log repository.AdminEventRepository,
) *RecoveryService {
	return &RecoveryService{users: users, passkeys: passkeys, sessions: sessions, log: log}
}

// RemoveSecondFactor turns an account's authenticator code off, deletes its
// passkeys and ends its sessions, so its password alone signs it in again.
//
// The passkeys go as well because they were most likely on the same phone, and a
// passkey on a phone somebody else now holds is a way in. The sessions go
// because that phone may still be signed in. Refused where there is nothing to
// remove, so a press that changed nothing does not read as one that did.
func (s *RecoveryService) RemoveSecondFactor(ctx context.Context, userID uint) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	held, err := s.passkeys.GetByUser(ctx, userID)
	if err != nil {
		return err
	}

	if !user.TOTPEnabled && len(held) == 0 {
		return apperror.Conflictf("%q has no second factor to remove", user.Email).
			WithCode("noSecondFactor", user.Email)
	}

	return s.clearSecondFactor(ctx, userID)
}

// RecoverSystemUser gives the built-in administrator a new password, made here
// and returned once, for the person at the server's command line.
//
// The account has to replace it at its next sign-in, its second factor is
// removed - the lost phone is as likely a reason to be here as the lost
// password - and every session it has ends. The password is set last, so a
// failure before it leaves the account opening with the password it had.
func (s *RecoveryService) RecoverSystemUser(ctx context.Context) (string, error) {
	user, err := s.users.GetByEmail(ctx, SystemUserEmail)
	if err != nil {
		return "", fmt.Errorf("find the built-in administrator %s: %w", SystemUserEmail, err)
	}

	if err := s.clearSecondFactor(ctx, user.ID); err != nil {
		return "", fmt.Errorf("remove the second factor of %s: %w", SystemUserEmail, err)
	}

	password := rand.Text()

	hash, err := security.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("make the new password: %w", err)
	}

	user.PasswordHash = hash
	user.MustChangePassword = true
	user.TOTPSecret, user.TOTPEnabled = "", false

	if _, err := s.users.Update(ctx, user); err != nil {
		return "", fmt.Errorf("give %s its new password: %w", SystemUserEmail, err)
	}

	event := &model.AdminEvent{At: time.Now().UTC(), Action: RecoveredAction, TargetID: user.ID}
	if err := s.log.Record(ctx, event); err != nil {
		return password, fmt.Errorf("%w: %w", ErrRecoveryNotLogged, err)
	}

	return password, nil
}

// clearSecondFactor is the part both ways share: no authenticator code, no
// passkeys, no session.
func (s *RecoveryService) clearSecondFactor(ctx context.Context, userID uint) error {
	if err := s.users.SetTOTP(ctx, userID, "", false); err != nil {
		return err
	}

	if err := s.passkeys.DeleteForUser(ctx, userID); err != nil {
		return err
	}

	return s.sessions.DeleteForUser(ctx, userID)
}
