package gsso

import (
	"os"
	"strings"

	xterm "github.com/charmbracelet/x/term"
)

const (
	ansiBoldGreen = "\033[1;32m"
	ansiReset     = "\033[0m"
)

// renderTable prints a header and rows in aligned columns, flush to the
// left. The last column is not padded. highlight marks rows to color when
// stdout is a terminal; the escape codes are applied after padding so they
// do not shift the columns. Pipes and NO_COLOR stay plain.
func renderTable(headers []string, rows [][]string, highlight []bool) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if n := len(row[i]); n > widths[i] {
				widths[i] = n
			}
		}
	}
	color := stdoutColorEnabled()
	var b strings.Builder
	write := func(cells []string, colored bool) {
		if colored && color {
			b.WriteString(ansiBoldGreen)
		}
		for i := range headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(cell)
			if i == len(headers)-1 {
				break
			}
			if pad := widths[i] - len(cell); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString("  ")
		}
		if colored && color {
			b.WriteString(ansiReset)
		}
		b.WriteByte('\n')
	}
	write(headers, false)
	for i, row := range rows {
		colored := i < len(highlight) && highlight[i]
		write(row, colored)
	}
	return b.String()
}

func stdoutColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return xterm.IsTerminal(os.Stdout.Fd())
}
