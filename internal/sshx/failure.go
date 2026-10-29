package sshx

import "strings"

// Failure is why ssh could not run a command, read from its messages (exit status 255). Its
// value is also the key of its explanation in the catalogs ("failure.timedOut").
type Failure string

const (
	FailureUnresolved     Failure = "unresolved"     // the host name does not resolve
	FailureRefused        Failure = "refused"        // nothing listens on that port
	FailureTimedOut       Failure = "timedOut"       // no answer, for example an IP filter
	FailureHostKeyUnknown Failure = "hostKeyUnknown" // not trusted yet
	FailureHostKeyChanged Failure = "hostKeyChanged" // differs from the trusted key
	FailureAuthRefused    Failure = "authRefused"    // no accepted key
	FailureKeyFileMissing Failure = "keyFileMissing" // the key file set for the host is missing
	FailureOther          Failure = "other"
)

// Failures lists every failure, for the checks that each one is explained.
var Failures = []Failure{
	FailureUnresolved, FailureRefused, FailureTimedOut, FailureHostKeyUnknown,
	FailureHostKeyChanged, FailureAuthRefused, FailureKeyFileMissing, FailureOther,
}

// Classify reads OpenSSH's messages; the most specific match wins.
func Classify(stderr []byte) Failure {
	msg := string(stderr)
	switch {
	case strings.Contains(msg, "REMOTE HOST IDENTIFICATION HAS CHANGED"),
		strings.Contains(msg, "has changed and you have requested strict checking"):
		return FailureHostKeyChanged
	case strings.Contains(msg, "Host key verification failed"):
		return FailureHostKeyUnknown
	case strings.Contains(msg, "Could not resolve hostname"):
		return FailureUnresolved
	case strings.Contains(msg, "Connection refused"):
		return FailureRefused
	case strings.Contains(msg, "timed out"):
		return FailureTimedOut
	case strings.Contains(msg, "Identity file") && strings.Contains(msg, "not accessible"):
		return FailureKeyFileMissing
	case strings.Contains(msg, "Permission denied"):
		return FailureAuthRefused
	}
	return FailureOther
}

// ConnectionError is returned when ssh itself fails; it matches ErrConnection.
type ConnectionError struct {
	Failure Failure
	Message string // what ssh printed
}

func (e *ConnectionError) Error() string {
	return ErrConnection.Error() + " (" + string(e.Failure) + "): " + e.Message
}

func (e *ConnectionError) Is(target error) bool { return target == ErrConnection }
