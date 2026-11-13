package service

import "context"

// The framework already measures the machinery: a histogram per HTTP request, a
// histogram per SQL query, a gauge of goroutines. None of that says whether the
// application is doing its job. A deployment can serve every request in
// milliseconds while nobody has been able to sign in since the directory changed,
// and app_http_response looks the same either way.
//
// So these three measure the work rather than the plumbing, and each one is here
// because somebody would act on it:
//
//   - refused sign-ins, which is either a directory that has stopped answering
//     or somebody working through a password list;
//   - accounts the directory synchronisation creates and deletes, which is the
//     one operation here that removes people, leaving their tickets without an
//     owner;
//   - what each scheduled synchronisation came to, because GoFr counts a job
//     that returned as a success, and a refused run returns like one that worked.
//
// Deliberately absent: anything labelled with a user, an address or a queue
// name. A label is a time series, and one per person is both a memory leak in
// the collector and a list of who works here, published on a port that asks for
// no password.
const (
	// MetricSignInFailures counts refused sign-ins, labelled "reason" with a
	// fixed handful of values.
	MetricSignInFailures = "gts_signin_failures_total"

	// MetricDirectoryAccounts counts what a synchronisation did, labelled
	// "action" with "created" or "deleted".
	MetricDirectoryAccounts = "gts_directory_accounts_total"

	// MetricDirectoryRuns counts what each scheduled synchronisation came to,
	// labelled "outcome" with one of the DirectoryRun values.
	MetricDirectoryRuns = "gts_directory_scheduled_runs_total"
)

// What a scheduled synchronisation came to. A closed set, because it is a label.
const (
	DirectoryRunCompleted = "completed"
	DirectoryRunRefused   = "refused"
	DirectoryRunFailed    = "failed"
)

// Reasons a sign-in was refused. A closed set, because it is a label.
const (
	SignInFailureCredentials = "credentials"
	SignInFailureDirectory   = "directory"
	SignInFailureTOTP        = "totp"

	// SignInFailureDatabase is a sign-in this installation could not answer
	// because its own database did not: counted as credentials, it looked like
	// everybody mistyping their password at once.
	SignInFailureDatabase = "database"
)

// Recorder is the part of the framework's metrics manager this application
// records through.
//
// Declared here rather than imported so the application layer keeps not knowing
// about the framework - the manager satisfies this structurally, so there is no
// adapter to keep in step either.
type Recorder interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

// metrics is embedded by the services that record, so each one gets the nil
// check once rather than at every call site.
//
// Nil is the ordinary state in a unit test, and a test that has to supply a
// metrics recorder to exercise a sign-in rule is a test about the wrong thing.
type metrics struct {
	recorder Recorder
}

func (m metrics) count(ctx context.Context, name string, labels ...string) {
	if m.recorder != nil {
		m.recorder.IncrementCounter(ctx, name, labels...)
	}
}
