package model

import "time"

// PushSubscription is a browser somebody asked for the reminders to be pushed
// to, as the browser handed it over: its push service's address, the two keys a
// message is encrypted for, and the address it reached the installation at,
// which each message is signed with as whom the push service may contact.
//
// A browser's push channel belongs to its profile and the installation's address
// rather than to an account, so somebody subscribing a browser another account
// had subscribed takes it over.
type PushSubscription struct {
	ID        uint
	UserID    uint
	Endpoint  string
	P256dh    string
	Auth      string
	Origin    string
	CreatedAt time.Time
}
