// Package backup is the file a TimePulse backup travels in: compressed, and then
// encrypted with a passphrase the server holds and the file does not.
//
// What is inside is the business of whoever writes it - the database's rows, read
// by the persistence layer - and this package only seals it: gzip, then
// AES-256-GCM under a key scrypt derives from the passphrase and a salt kept in
// the file's header, which the cipher authenticates as well. A file opened with
// the wrong passphrase, or changed by a single bit, is refused as a whole rather
// than read in part. And what a file decompresses to is bounded, along with what
// its header may ask the key derivation to cost, so a crafted file can neither
// fill a disk nor hold a processor.
package backup
