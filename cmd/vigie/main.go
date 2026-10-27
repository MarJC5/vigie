// Command vigie monitors and updates WordPress sites over SSH and WP-CLI.
package main

import (
	"fmt"

	"github.com/MarJC5/vigie/internal/buildinfo"
)

func main() {
	fmt.Println("vigie", buildinfo.Version())
}
