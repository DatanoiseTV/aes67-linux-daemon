//go:build linux

package privileges

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

// Drop switches the process to run as the specified user.
// Must be called after opening privileged resources (netlink sockets).
func Drop(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	// Set supplementary groups
	gids, _ := u.GroupIds()
	var intGids []int
	for _, g := range gids {
		id, _ := strconv.Atoi(g)
		intGids = append(intGids, id)
	}
	if len(intGids) > 0 {
		syscall.Setgroups(intGids)
	}

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid(%d): %w", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid(%d): %w", uid, err)
	}
	return nil
}

// IsRoot returns true if running as root.
func IsRoot() bool {
	return syscall.Getuid() == 0
}
