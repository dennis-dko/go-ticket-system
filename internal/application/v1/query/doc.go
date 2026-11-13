// Package query holds the read requests the application layer accepts, and the
// results they come back as.
//
// Separate from command because a read may carry a question a write does not:
// who is asking. Where it does, that id decides visibility rather than
// filtering - a record the viewer may not see is answered as not found, not as
// refused. And where a query also names whose records are wanted, that is a
// second and different id, kept in a field of its own: one id doing both jobs
// is how an answer ends up including somebody else's records.
package query
