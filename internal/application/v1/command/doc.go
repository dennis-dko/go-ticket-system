// Package command holds the write requests the application layer accepts -
// what a caller is asking to create, change or delete, separated from however
// it arrived.
//
// One rule shapes most of these types, and it is the reason the package is
// worth a comment. A partial update built from pointers can say "leave this
// alone" and "set it to this", and nothing else. A field that has to be
// emptiable therefore needs its third state said out loud, in a field of its
// own. Never overload nil to mean "clear": every caller that stays silent about
// a field relies on nil meaning silence, the spreadsheet import among them.
package command
