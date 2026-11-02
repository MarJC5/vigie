package main

import "golang.org/x/sys/unix"

// termiosRequest reads a terminal's settings, which only a terminal has.
const termiosRequest = unix.TIOCGETA
