package exchange

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/MarJC5/vigie/internal/model"
)

// ErrNotACSV is returned for a CSV file Vigie cannot read: no header, a column it does not know,
// a column it needs missing, quotes left open.
var ErrNotACSV = errors.New("not a site list Vigie reads")

// CSVReason tells why a line of a CSV file is refused.
type CSVReason string

const (
	CSVColumns        CSVReason = "columns"         // not as many values as the header
	CSVNoPath         CSVReason = "no-path"         // the WordPress root is missing
	CSVNoConnection   CSVReason = "no-connection"   // neither an alias nor a host
	CSVBothConnection CSVReason = "both-connection" // an alias and a host
	CSVNoUser         CSVReason = "no-user"         // a host without a user
	CSVBadPort        CSVReason = "bad-port"
	CSVBadColor       CSVReason = "bad-color"
	CSVBadName        CSVReason = "bad-name"     // empty or with a slash
	CSVBadFolder      CSVReason = "bad-folder"   // an empty part, such as "Clients//Valais"
	CSVOtherFolder    CSVReason = "other-folder" // its account is in another folder on an earlier line
	CSVSameSite       CSVReason = "same-site"    // its account and path are on an earlier line
)

// CSVRefusal is a line of a CSV file left out, with why, and the value at fault when there is one.
type CSVRefusal struct {
	Line   int
	Reason CSVReason
	Value  string
}

// The columns of a site list: a connection (alias, or host, port and user), the site's path and
// address, and optionally its folder, name, color and notes.
var csvColumns = []string{"folder", "alias", "host", "port", "user", "path", "url", "name", "color", "notes"}

// ReadCSV reads a site list: a header line, then one line per site. Values are separated by commas,
// or by semicolons when the header has more of them (as spreadsheets write in French), quoted as
// RFC 4180 says; a leading byte order mark is skipped. Folders are paths ("Clients/Valais");
// sites of the same account share one host, named after its alias or its hostname. What the file
// gives becomes an export with new IDs, to import with its preview; the lines refused are listed.
func ReadCSV(r io.Reader) (File, []CSVRefusal, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return File{}, nil, err
	}
	if len(data) > maxSize {
		return File{}, nil, fmt.Errorf("larger than %d MB: %w", maxSize>>20, ErrNotACSV)
	}
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))
	header, _, _ := bufio.NewReader(bytes.NewReader(data)).ReadLine()
	reader := csv.NewReader(bytes.NewReader(data))
	if bytes.Count(header, []byte(";")) > bytes.Count(header, []byte(",")) {
		reader.Comma = ';'
	}
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	names, err := reader.Read()
	if err != nil {
		return File{}, nil, fmt.Errorf("header: %w", ErrNotACSV)
	}
	columns := map[string]int{}
	for i, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if !slices.Contains(csvColumns, name) {
			return File{}, nil, fmt.Errorf("column %q: %w", name, ErrNotACSV)
		}
		if _, twice := columns[name]; twice {
			return File{}, nil, fmt.Errorf("column %q twice: %w", name, ErrNotACSV)
		}
		columns[name] = i
	}
	_, hasAlias := columns["alias"]
	_, hasHost := columns["host"]
	_, hasUser := columns["user"]
	if _, ok := columns["path"]; !ok || (!hasAlias && !(hasHost && hasUser)) {
		return File{}, nil, fmt.Errorf("a path, and an alias or a host and user, are needed: %w", ErrNotACSV)
	}
	b := csvBuilder{
		file:    File{Folders: []Folder{}, Hosts: []Host{}, Sites: []Site{}, Ignores: []Ignore{}},
		folders: map[string]string{}, hosts: map[string]csvHost{}, sites: map[string]bool{},
	}
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return File{}, nil, fmt.Errorf("%w: %w", ErrNotACSV, err)
		}
		line, _ := reader.FieldPos(0)
		if len(record) != len(names) {
			b.refused = append(b.refused, CSVRefusal{Line: line, Reason: CSVColumns})
			continue
		}
		value := func(name string) string {
			if i, ok := columns[name]; ok {
				return strings.TrimSpace(record[i])
			}
			return ""
		}
		if refusal := b.add(value); refusal != nil {
			refusal.Line = line
			b.refused = append(b.refused, *refusal)
		}
	}
	return b.file, b.refused, nil
}

type csvHost struct {
	id, folder string
	sites      int
}

type csvBuilder struct {
	file    File
	refused []CSVRefusal
	folders map[string]string  // IDs by path
	hosts   map[string]csvHost // by connection
	sites   map[string]bool    // by connection and path
}

// add makes the site of a line, with its host and folders when they are new.
func (b *csvBuilder) add(value func(string) string) *CSVRefusal {
	refuse := func(reason CSVReason, v string) *CSVRefusal { return &CSVRefusal{Reason: reason, Value: v} }
	alias, hostname, user, sitePath := value("alias"), value("host"), value("user"), value("path")
	switch {
	case sitePath == "":
		return refuse(CSVNoPath, "")
	case alias == "" && hostname == "":
		return refuse(CSVNoConnection, "")
	case alias != "" && hostname != "":
		return refuse(CSVBothConnection, "")
	case hostname != "" && user == "":
		return refuse(CSVNoUser, "")
	}
	port := 0
	if p := value("port"); p != "" && alias == "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return refuse(CSVBadPort, p)
		}
		port = n
	}
	color := model.ColorNone.String()
	if c := strings.ToLower(value("color")); c != "" {
		if !knownColor(c) {
			return refuse(CSVBadColor, value("color"))
		}
		color = c
	}
	name := value("name")
	if name == "" {
		name = siteName(value("url"), sitePath)
	}
	if model.ValidateName(name) != nil {
		return refuse(CSVBadName, name)
	}
	folder := strings.Trim(value("folder"), "/")
	for part := range strings.SplitSeq(folder, "/") {
		if folder != "" && strings.TrimSpace(part) == "" {
			return refuse(CSVBadFolder, value("folder"))
		}
	}
	key := "alias " + alias
	if alias == "" {
		key = fmt.Sprintf("%s %d %s", strings.ToLower(hostname), max(port, 22), user)
	}
	host, known := b.hosts[key]
	if known && host.folder != folder {
		return refuse(CSVOtherFolder, value("folder"))
	}
	siteKey := key + "\n" + strings.TrimSuffix(sitePath, "/")
	if b.sites[siteKey] {
		return refuse(CSVSameSite, sitePath)
	}
	if !known {
		hostName := alias
		if alias == "" {
			hostName = hostname
		}
		host = csvHost{id: model.NewID().String(), folder: folder}
		b.file.Hosts = append(b.file.Hosts, Host{ID: host.id, Folder: b.folder(folder), Name: hostName, Alias: alias,
			Hostname: hostname, Port: port, User: user, Color: model.ColorNone.String(), TrustedKeys: []HostKey{},
			Position: len(b.file.Hosts)})
	}
	b.sites[siteKey] = true
	b.file.Sites = append(b.file.Sites, Site{ID: model.NewID().String(), Host: host.id, Name: name, Path: sitePath,
		URL: value("url"), Color: color, Notes: value("notes"), Tags: []string{}, Subsites: []Subsite{}, Position: host.sites})
	host.sites++
	b.hosts[key] = host
	return nil
}

// folder returns the ID of the folder at a path, made with its parents when new; "" is the top.
func (b *csvBuilder) folder(p string) string {
	if p == "" {
		return ""
	}
	if id, ok := b.folders[p]; ok {
		return id
	}
	parent, name := "", p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		parent, name = b.folder(p[:i]), p[i+1:]
	}
	id := model.NewID().String()
	b.folders[p] = id
	b.file.Folders = append(b.file.Folders, Folder{ID: id, Parent: parent, Name: strings.TrimSpace(name),
		Color: model.ColorNone.String(), Position: len(b.file.Folders)})
	return id
}

// siteName is the address's host name, or else the last part of the path.
func siteName(address, sitePath string) string {
	if u, err := url.Parse(address); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return path.Base(strings.TrimSuffix(sitePath, "/"))
}
