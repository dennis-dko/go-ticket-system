// Package service holds the domain rules - the answers that stay the same
// whoever is asking and however the question arrived.
//
// It is deliberately small. Most of what happens between a request and a row is
// orchestration, and that lives in internal/application; what is here is the
// handful of decisions it would be wrong to make twice. It began with the rule
// about which records a caller may see, because the application layer had its
// own copy of it, word for word, and a rule agreed upon in two places is one
// that holds only until somebody widens one of them.
//
// That rule is the one to keep in mind before adding anything here: a record the
// caller may not see is not found, never refused. The difference between 403
// and 404 is itself the leak - answering the wrong one says that the record
// exists and that somebody has it.
package service
