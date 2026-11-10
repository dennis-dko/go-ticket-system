// Package mobile is the Android app: a Gradle build of its own in this
// directory, and the tests that hold it to the rest of the application.
//
// The app is an installation's own web interface in a WebView, signed in with a
// personal token that it hands over for a session, and it is connected by the
// code a new token is shown with. Besides the page it shows the installation's
// reminders as notifications of its own, asking with the same token. It is
// Kotlin, built in the image this directory's Dockerfile describes, so the Go
// toolchain compiles nothing here. What it reads of the app is what can drift
// from the rest unnoticed: the dictionaries' keys, the English the page already
// says, how a refusal's values are written, the address a token's code holds,
// the bridges the page reaches the app by, and the directory the dictionaries
// are copied from.
package mobile
