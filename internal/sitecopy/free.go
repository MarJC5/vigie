//go:build darwin || linux

package sitecopy

import "syscall"

// DiskFree answers the free space a user can write on the disk of dir, in bytes.
func DiskFree(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
