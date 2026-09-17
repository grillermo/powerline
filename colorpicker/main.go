// Command colorpicker lets the user pick the hostname segment color with
// chicle and prints the chosen 256-color index to stdout for ./build.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/grillermo/chicle"
)

// palette mirrors hostnameColor() in powerline-zsh.go; keep them in sync.
var palette = []int{
	17, 18, 19, 21,
	22, 23, 24, 28,
	52, 88, 124,
	53, 55, 56, 90, 91,
	58, 94, 130,
	161,
}

func main() {
	host, _ := os.Hostname()
	if idx := strings.Index(host, "."); idx != -1 {
		host = host[:idx]
	}

	rows := make([]chicle.Row, len(palette))
	for i, c := range palette {
		n := strconv.Itoa(c)
		// The swatch carries raw ANSI, so it must stay in the last column:
		// chicle pads fixed-width columns by byte count.
		swatch := fmt.Sprintf("\x1b[38;5;15;48;5;%dm %s \x1b[0m", c, host)
		rows[i] = chicle.Row{Key: n, Cols: []string{n, swatch}}
	}

	color, err := chicle.Run(chicle.Config{
		Title:   "Hostname color for powerline-zsh",
		Columns: []chicle.Column{{Title: "Color", Width: 5}, {Title: "Preview"}},
		Rows:    rows,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "colorpicker:", err)
		os.Exit(1)
	}
	fmt.Println(color)
}
