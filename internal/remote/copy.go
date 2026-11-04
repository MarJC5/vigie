package remote

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
)

// The commands of a copy of a site on the user's computer (ADR-0041). They only read: the sizes,
// the files of a part, a tar archive and a database dump written to the standard output.

// CopyTools asks which find and tar the account has: a copy is split by size only with GNU find
// (-printf) and GNU tar (--null -T -). Each answers its first line, or nothing.
func CopyTools() string {
	return "(find --version 2>/dev/null | head -n 1); echo ---; (tar --version 2>/dev/null | head -n 1); true"
}

// Tools are the find and tar of an account, as a copy needs them.
type Tools struct {
	GNUFind bool
	GNUTar  bool
}

// ParseCopyTools reads CopyTools's answer. BusyBox's find answers nothing to --version, and an
// account can have GNU tar without GNU find (the WP-CLI image on Alpine).
func ParseCopyTools(out []byte) Tools {
	find, tar, _ := strings.Cut(string(out), "---")
	return Tools{GNUFind: strings.Contains(find, "GNU findutils"), GNUTar: strings.Contains(tar, "GNU tar")}
}

// ContentSizes answers the size of each folder of wp-content, in kilobytes, then the size of the
// whole: what is not in a folder is the difference.
func ContentSizes(content string) (string, error) {
	if err := folder(content); err != nil {
		return "", err
	}
	q, err := quoted(content)
	if err != nil {
		return "", err
	}
	return "cd " + q + " && du -sk -- */ 2>/dev/null; echo ---; du -sk . 2>/dev/null; true", nil
}

// FolderSize is a folder of wp-content and its size on disk.
type FolderSize struct {
	Name  string
	Bytes int64
}

// ContentSize is what ContentSizes answered: the folders, the largest first, and the whole.
type ContentSize struct {
	Folders []FolderSize
	Total   int64
}

// ParseContentSizes reads ContentSizes's answer; without the size of the whole, the answer is cut.
func ParseContentSizes(out []byte) (ContentSize, error) {
	folders, whole, ok := strings.Cut(string(out), "---")
	if !ok {
		return ContentSize{}, fmt.Errorf("content sizes %.80q: no total", out)
	}
	var size ContentSize
	for _, line := range strings.Split(folders, "\n") {
		kb, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		n, err := strconv.ParseInt(kb, 10, 64)
		name = strings.TrimSuffix(name, "/")
		if !ok || err != nil || n < 0 || name == "" || strings.Contains(name, "/") {
			continue
		}
		size.Folders = append(size.Folders, FolderSize{Name: name, Bytes: n * 1024})
	}
	kb, _, _ := strings.Cut(strings.TrimSpace(whole), "\t")
	total, err := strconv.ParseInt(kb, 10, 64)
	if err != nil || total < 0 {
		return ContentSize{}, fmt.Errorf("content sizes %.80q: total %q", out, kb)
	}
	size.Total = total * 1024
	slices.SortStableFunc(size.Folders, func(a, b FolderSize) int { return int(b.Bytes>>10 - a.Bytes>>10) })
	return size, nil
}

// ListFiles lists the files of a folder with their sizes, the folders skip left out (each inside
// dir): "<bytes> <path from dir>", each ended by a NUL, so that any name goes through (GNU find).
func ListFiles(dir string, skip []string) (string, error) {
	if err := folder(dir); err != nil {
		return "", err
	}
	q, err := quoted(dir)
	if err != nil {
		return "", err
	}
	cmd := "find " + q
	if len(skip) > 0 {
		var paths []string
		for _, s := range skip {
			if err := folder(s); err != nil {
				return "", err
			}
			if !strings.HasPrefix(s, dir+"/") {
				return "", fmt.Errorf("%q outside %q: %w", s, dir, ErrBadPath)
			}
			p, err := quoted(s)
			if err != nil {
				return "", err
			}
			paths = append(paths, "-path "+p)
		}
		cmd += " '(' " + strings.Join(paths, " -o ") + " ')' -prune -o"
	}
	format, err := quoted(`%s %P\0`)
	return cmd + " -type f -printf " + format, err
}

// File is a file of a part of a site: its path from the part's folder, and its size.
type File struct {
	Name  string
	Bytes int64
}

// ParseFileList reads ListFiles's answer.
func ParseFileList(out []byte) ([]File, error) {
	var files []File
	for _, entry := range bytes.Split(out, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		size, name, ok := strings.Cut(string(entry), " ")
		n, err := strconv.ParseInt(size, 10, 64)
		if !ok || err != nil || n < 0 || name == "" || strings.HasPrefix(name, "/") || path.IsAbs(name) {
			return nil, fmt.Errorf("file list entry %.80q: unexpected", entry)
		}
		files = append(files, File{Name: name, Bytes: n})
	}
	return files, nil
}

// ArchiveFiles writes a tar archive of the files whose names it reads on its standard input, each
// ended by a NUL, from the folder dir, to its standard output. A file gone since it was listed is
// skipped with a warning (--ignore-failed-read), not a failure of the whole archive.
func ArchiveFiles(dir string) (string, error) {
	if err := folder(dir); err != nil {
		return "", err
	}
	q, err := quoted(dir)
	return "cd " + q + " && tar -cf - --ignore-failed-read --no-recursion --null -T -", err
}

// DumpCompressed compresses on the fly the dump that the WP-CLI command wp writes to its standard
// output. A pipe answers gzip's exit status, not the dump's: the dump's is written on the standard
// error, read by ParseDumpExit.
func DumpCompressed(wp string) string {
	return "{ " + wp + `; echo "vigie-dump-exit $?" >&2; } | gzip -c`
}

// ParseDumpExit reads the dump's exit status that DumpCompressed wrote on the standard error.
func ParseDumpExit(stderr []byte) (int, bool) {
	for _, line := range strings.Split(string(stderr), "\n") {
		if code, ok := strings.CutPrefix(strings.TrimSpace(line), "vigie-dump-exit "); ok {
			n, err := strconv.Atoi(code)
			return n, err == nil
		}
	}
	return 0, false
}
