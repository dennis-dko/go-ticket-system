// Package model is the vocabulary the rest of this application is written in:
// the entities, the value types they store, and what is true about them
// wherever they are read.
//
// It imports nothing but the standard library, and that is the shape of the
// architecture rather than an accident of this package. internal/domain may
// import neither internal/infrastructure nor internal/interface, so the layer
// that holds the rules can be read and tested without a database, an HTTP
// request, or a clock it was not handed.
//
// One decision here is worth knowing before changing anything, because it was
// made after the obvious version went wrong.
//
// A stored date is a calendar day and not an instant - CalendarDay, midnight
// UTC, carrying no zone at all. Which day it is gets read in the reader's own
// location; what is stored knows nothing about location. The version that kept
// the zone stored 15 July as midnight in Berlin from the clock and as midnight
// UTC from a form, and the two were counted as two days.
package model
