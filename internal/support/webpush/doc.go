// Package webpush sends a notification to a browser through that browser's push
// service, as the Web Push standards say: the message encrypted for that browser
// alone (RFC 8291) and signed with the installation's own key (RFC 8292), so the
// push service carries it without being able to read it, and accepts it only from
// whoever holds the key the browser subscribed with.
//
// It is the standard library and nothing else. The protocol is small enough to
// hold against the RFC's own worked example, which the tests do byte for byte,
// and a dependency for it would be one more thing that sees every reminder.
//
// What it does not decide is when to send, or to whom: that is the reminders',
// and the installation's switch that allows it at all.
package webpush
