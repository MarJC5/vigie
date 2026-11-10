// Package errorlog reads the lines of a site's PHP error log that the server let through (fatal
// errors and warnings, and the "thrown in" line under an uncaught exception), groups them, and masks
// what they may hold of people and secrets. It only reads text: no connection, no storage. What it
// returns is all that leaves Vigie's service, for the window, the command and an assistant alike.
//
// A log is written in part by anyone (a requested address, a form field, a plugin's message) and a
// line can be forged: the place an error names is where it was raised, a lead and not a verdict, and
// a mask lowers the risk without removing it.
package errorlog

import (
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Type is the type PHP gives an error.
type Type string

const (
	Fatal       Type = "fatal"
	Parse       Type = "parse"
	Recoverable Type = "recoverable"
	Warning     Type = "warning"
)

// Error tells an error from a warning: errors come first.
func (t Type) Error() bool { return t != Warning }

// Kind is what the file of an error belongs to.
type Kind string

const (
	Plugin    Kind = "plugin"
	Theme     Kind = "theme"
	MustUse   Kind = "mu-plugin"
	WordPress Kind = "wordpress"
	Site      Kind = "site"    // another file of the site
	Outside   Kind = "outside" // a file outside the site's folder
	Unknown   Kind = "unknown" // the line read does not say where
)

// Group is the errors of one type, raised at one place, whose messages differ only by their variable
// parts. Each field is what leaves the service: adding one is a decision about what is shared.
type Group struct {
	Type    Type
	Kind    Kind
	Slug    string // the plugin's, theme's or must-use plugin's
	File    string // relative to the site's folder; "[path]" outside it, "" when not said
	Line    int
	Message string // the latest one's, masked, on one line, MaxMessage characters at most
	Count   int
	First   time.Time // UTC
	Last    time.Time // UTC
}

// Summary is what a read found.
type Summary struct {
	Groups []Group   // worst first, then newest
	Lines  int       // the errors and warnings read
	Since  time.Time // the oldest of them, UTC; how far back the read goes
}

// MaxMessage bounds a message, in characters.
const MaxMessage = 200

var (
	// "[d-M-Y H:i:s e] PHP <type>:  <message>", the form PHP's error_log writes to a file.
	errorLine = regexp.MustCompile(`^\[(\d{2}-[A-Z][a-z]{2}-\d{4} \d{2}:\d{2}:\d{2}) ([^\]]+)\] PHP (Fatal error|Parse error|Recoverable fatal error|Warning):\s+(.*)$`)
	// The place at the end of a message: "in <file> on line <N>", or "in <file>:<N>" for an uncaught
	// exception. The last " in " wins, a message can hold others.
	place = regexp.MustCompile(`^(.*) in (\S.*?)(?: on line |:)(\d+)$`)
	// Under an uncaught exception, with no date and no argument.
	thrownIn = regexp.MustCompile(`^  thrown in (\S.*) on line (\d+)$`)
)

var types = map[string]Type{"Fatal error": Fatal, "Parse error": Parse, "Recoverable fatal error": Recoverable, "Warning": Warning}

type entry struct {
	typ     Type
	at      time.Time
	file    string // as PHP wrote it, absolute
	line    int
	message string // as PHP wrote it
}

// Read parses the lines the server sent, groups them and masks them; root is the site's real folder,
// as PHP writes its paths. A line in another form is left out.
func Read(lines []string, root string) Summary {
	root = strings.TrimRight(root, "/")
	var entries []entry
	for _, raw := range lines {
		raw = strings.TrimRight(raw, "\r")
		if m := thrownIn.FindStringSubmatch(raw); m != nil {
			// The place of the exception just above, when its own line did not say it.
			if n := len(entries); n > 0 && entries[n-1].file == "" && strings.HasPrefix(entries[n-1].message, "Uncaught ") {
				entries[n-1].file, entries[n-1].line = m[1], atoi(m[2])
			}
			continue
		}
		m := errorLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		e := entry{typ: types[m[3]], at: logTime(m[1], m[2]), message: m[4]}
		if p := place.FindStringSubmatch(e.message); p != nil {
			e.message, e.file, e.line = p[1], p[2], atoi(p[3])
		}
		entries = append(entries, e)
	}
	return summarize(entries, root)
}

func summarize(entries []entry, root string) Summary {
	s := Summary{Lines: len(entries)}
	byKey := map[string]int{}
	for _, e := range entries {
		if s.Since.IsZero() || e.at.Before(s.Since) {
			s.Since = e.at
		}
		message := Mask(e.message, root)
		key := string(e.typ) + "\x00" + e.file + "\x00" + strconv.Itoa(e.line) + "\x00" + variablePartsRemoved(message)
		i, ok := byKey[key]
		if !ok {
			kind, slug, file := belongsTo(e.file, root)
			s.Groups = append(s.Groups, Group{Type: e.typ, Kind: kind, Slug: slug, File: file, Line: e.line, First: e.at, Last: e.at})
			i = len(s.Groups) - 1
			byKey[key] = i
		}
		g := &s.Groups[i]
		g.Count++
		g.First = minTime(g.First, e.at)
		if !e.at.Before(g.Last) {
			g.Last, g.Message = e.at, oneLine(message)
		}
	}
	slices.SortStableFunc(s.Groups, func(a, b Group) int {
		if a.Type.Error() != b.Type.Error() {
			if a.Type.Error() {
				return -1
			}
			return 1
		}
		return b.Last.Compare(a.Last)
	})
	return s
}

// logTime reads the time PHP wrote, in its zone; a zone this computer does not know is read as UTC
// rather than losing the line.
func logTime(at, zone string) time.Time {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}
	t, err := time.ParseInLocation("02-Jan-2006 15:04:05", at, loc)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// belongsTo names what a file belongs to, from its path in the site: plugins, themes and must-use
// plugins by their folder, wherever wp-content is; WordPress by its own folders and files.
func belongsTo(file, root string) (Kind, string, string) {
	if file == "" {
		return Unknown, "", ""
	}
	rel, ok := strings.CutPrefix(file, root+"/")
	if !ok {
		return Outside, "", masked["path"]
	}
	parts := strings.Split(rel, "/")
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "mu-plugins":
			return MustUse, strings.TrimSuffix(parts[i+1], ".php"), rel
		case "plugins", "themes":
			if i+2 < len(parts) {
				if parts[i] == "plugins" {
					return Plugin, parts[i+1], rel
				}
				return Theme, parts[i+1], rel
			}
		}
	}
	if parts[0] == "wp-admin" || parts[0] == "wp-includes" || (len(parts) == 1 && (strings.HasPrefix(rel, "wp-") || rel == "index.php" || rel == "xmlrpc.php")) {
		return WordPress, "", rel
	}
	return Site, "", rel
}

// masked are the words that stand for what a mask hid, the same in every language.
var masked = map[string]string{"email": "[email]", "ip": "[ip]", "query": "[query]", "token": "[token]", "path": "[path]"}

var (
	email = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
	// A query after an address or a file: "?" right after a word, up to a space or a quote.
	query = regexp.MustCompile(`([^\s?])\?[^\s"'()<>]+`)
	ipv4  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// What may be an IPv6 address: hexadecimal digits and at least two colons, checked by net.
	ipv6 = regexp.MustCompile(`[0-9A-Fa-f]*:[0-9A-Fa-f:]*:[0-9A-Fa-f]*`)
	hex  = regexp.MustCompile(`\b[0-9A-Fa-f]{16,}\b`)
	// What may be a token in base64: long, without slash (a path's), checked in maskToken.
	base64 = regexp.MustCompile(`[A-Za-z0-9+_-]{20,}={0,2}`)
	// An absolute path of two parts or more, not in an address (after ":" or "/"), up to its query.
	path = regexp.MustCompile(`(^|[\s"'(=,\[])(/[^\s"'(),:?\[\]]+/[^\s"'(),:?\[\]]*)`)
)

// Mask hides in a message the e-mail addresses, the IP addresses, the queries of addresses, the
// tokens and the paths: the site's own made relative to its folder, the others replaced. A mask will
// miss things, a name or a cut address.
func Mask(s, root string) string {
	s = strings.ToValidUTF8(s, "�")
	if root = strings.TrimRight(root, "/"); root != "" {
		s = strings.ReplaceAll(s, root+"/", "")
	}
	s = email.ReplaceAllString(s, masked["email"])
	s = query.ReplaceAllString(s, "${1}?"+masked["query"])
	s = ipv4.ReplaceAllStringFunc(s, func(m string) string {
		if net.ParseIP(m) == nil {
			return m
		}
		return masked["ip"]
	})
	s = ipv6.ReplaceAllStringFunc(s, func(m string) string {
		if strings.Count(m, ":") < 2 || net.ParseIP(m) == nil {
			return m
		}
		return masked["ip"]
	})
	s = hex.ReplaceAllString(s, masked["token"])
	s = base64.ReplaceAllStringFunc(s, maskToken)
	s = path.ReplaceAllString(s, "${1}"+masked["path"])
	return s
}

// maskToken hides a word that reads as random: upper and lower case letters, and two digits or more.
// A long name of a class has seldom more than one digit.
func maskToken(w string) string {
	var upper, lower, digits int
	for _, r := range w {
		switch {
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lower++
		case unicode.IsDigit(r):
			digits++
		}
	}
	if upper > 0 && lower > 0 && digits >= 2 {
		return masked["token"]
	}
	return w
}

var (
	quotedDouble = regexp.MustCompile(`"[^"]*"`)
	quotedSingle = regexp.MustCompile(`'[^']*'`)
	digits       = regexp.MustCompile(`\d+`)
)

// variablePartsRemoved is a message without what changes from one time to the next at the same
// place: quoted values and numbers.
func variablePartsRemoved(s string) string {
	s = quotedDouble.ReplaceAllString(s, `""`)
	s = quotedSingle.ReplaceAllString(s, `''`)
	return digits.ReplaceAllString(s, "0")
}

// oneLine puts a message on one line and cuts it to MaxMessage characters.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= MaxMessage {
		return s
	}
	return string([]rune(s)[:MaxMessage-1]) + "…"
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
