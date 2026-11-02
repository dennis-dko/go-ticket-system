// Package mobile is the Android app: a Gradle build of its own in this
// directory, and the tests that hold it to the rest of the application.
//
// The app does what the terminal client does - the stopwatch, today's entries,
// booking by hand - by asking an installation's API with a personal token, and
// it is connected by scanning the code a new token is shown with. It is Kotlin,
// built in the image this directory's Dockerfile describes, so the Go toolchain
// compiles nothing here. What it reads of the app is what can drift from the
// rest unnoticed: the dictionaries' keys, the English the terminal client and
// the page already say, how a refusal's values are written, the address a
// token's code holds, and the directory the dictionaries are copied from.
package mobile
