package service

import (
	"context"

	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/support/apperror"
)

// RecoveryService is the way back into an account nobody can sign into.
//
// An administrator removes the second factor of somebody who lost the phone it
// was on: an authenticator code cannot be typed from a phone that is gone, and
// only its owner could turn it off, with a code from that phone.
type RecoveryService struct {
	users    repository.UserRepository
	passkeys repository.PasskeyRepository
	sessions repository.SessionRepository
}

// NewRecoveryService wires the repositories a recovery touches.
func NewRecoveryService(
	users repository.UserRepository,
	passkeys repository.PasskeyRepository,
	sessions repository.SessionRepository,
) *RecoveryService {
	return &RecoveryService{users: users, passkeys: passkeys, sessions: sessions}
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

// clearSecondFactor takes away every way in past the password: no authenticator
// code, no passkeys, no session.
func (s *RecoveryService) clearSecondFactor(ctx context.Context, userID uint) error {
	if err := s.users.SetTOTP(ctx, userID, "", false); err != nil {
		return err
	}

	if err := s.passkeys.DeleteForUser(ctx, userID); err != nil {
		return err
	}

	return s.sessions.DeleteForUser(ctx, userID)
}
