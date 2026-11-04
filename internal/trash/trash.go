// Package trash moves a file or folder to the trash of a Linux desktop, as the freedesktop.org
// Trash specification says (ADR-0041): the home trash, or the trash at the top of another disk.
package trash

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ErrNoTrash is returned when no trash can take the file: the window then offers to delete it for
// good.
var ErrNoTrash = errors.New("no trash can take it")

// Home is the user's home trash, in DataHome.
type Home struct {
	DataHome string // $XDG_DATA_HOME, or ~/.local/share
	Now      func() time.Time
}

// DataHome is the base folder of a user's data: $XDG_DATA_HOME when it is an absolute path, else
// ~/.local/share (XDG Base Directory specification).
func DataHome(home, xdg string) string {
	if filepath.IsAbs(xdg) {
		return xdg
	}
	return filepath.Join(home, ".local", "share")
}

// Put moves path to a trash: the home trash when path is on its disk, else the trash at the top of
// path's disk, $topdir/.Trash/$uid or $topdir/.Trash-$uid. When none can take it, ErrNoTrash, and
// path stays.
func (h Home) Put(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dev, err := device(path)
	if err != nil {
		return err
	}
	// On the home trash's disk, only the home trash.
	if d, err := device(existing(h.DataHome)); err == nil && d == dev {
		return h.put(filepath.Join(h.DataHome, "Trash"), path, path)
	}
	top := mountPoint(path, dev)
	rel, err := filepath.Rel(top, path)
	if err != nil {
		return err
	}
	uid := strconv.Itoa(os.Getuid())
	// (1) An administrator's $topdir/.Trash, sticky and not a link: the user's folder in it.
	if info, err := os.Lstat(filepath.Join(top, ".Trash")); err == nil && info.IsDir() && info.Mode()&os.ModeSticky != 0 {
		if err := h.put(filepath.Join(top, ".Trash", uid), path, rel); err == nil || !errors.Is(err, ErrNoTrash) {
			return err
		}
	}
	// (2) The user's own $topdir/.Trash-$uid.
	return h.put(filepath.Join(top, ".Trash-"+uid), path, rel)
}

// put moves path into the trash folder dir, its .trashinfo saying where it was (location, absolute
// or relative to the disk's top). A trash that cannot be made, or a move that cannot be done, is
// ErrNoTrash.
func (h Home) put(dir, path, location string) error {
	files, info := filepath.Join(dir, "files"), filepath.Join(dir, "info")
	for _, d := range []string{files, info} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("%w: %w", ErrNoTrash, err)
		}
	}
	base := filepath.Base(path)
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = base + "." + strconv.Itoa(i)
		}
		// The .trashinfo is made first, and only if it is new: it reserves the name.
		f, err := os.OpenFile(filepath.Join(info, name+".trashinfo"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrNoTrash, err)
		}
		escaped := (&url.URL{Path: location}).EscapedPath()
		_, err = fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", escaped, h.Now().Format("2006-01-02T15:04:05"))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(path, filepath.Join(files, name))
		}
		if err != nil {
			os.Remove(filepath.Join(info, name+".trashinfo"))
			return fmt.Errorf("%w: %w", ErrNoTrash, err)
		}
		return nil
	}
}

// device is the disk a path is on.
func device(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil //nolint:unconvert // its type differs between systems
}

// existing is path, or its nearest parent that exists.
func existing(path string) string {
	for p := path; ; p = filepath.Dir(p) {
		if _, err := os.Stat(p); err == nil || p == filepath.Dir(p) {
			return p
		}
	}
}

// mountPoint is the top of the disk path is on: its highest parent on the same disk.
func mountPoint(path string, dev uint64) string {
	top := path
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		if d, err := device(p); err != nil || d != dev {
			return top
		}
		top = p
		if p == filepath.Dir(p) || strings.TrimSuffix(p, "/") == "" {
			return top
		}
	}
}
