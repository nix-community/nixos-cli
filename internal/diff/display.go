package diff

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
)

func displayDiffResults(closureDiff *ClosureDiff) {
	fmt.Println("Closure Comparison:")
	fmt.Println(strings.Repeat("=", 19))

	added := 0
	removed := 0
	changed := 0

	for _, diff := range closureDiff.Diffs {
		switch diff.Change {
		case ChangeTypeAdd:
			added++
		case ChangeTypeRemove:
			removed++
		case ChangeTypeChange:
			changed++
		}
	}

	fmt.Println("\nPackages:")
	fmt.Printf("  + %d added\n", added)
	fmt.Printf("  - %d removed\n", removed)
	fmt.Printf("  ~ %d changed\n", changed)

	fmt.Println("\nSize:")
	if closureDiff.OldSize == closureDiff.NewSize {
		fmt.Println("  (no change)")
	} else {
		oldSize := formatSize(closureDiff.OldSize)
		newSize := formatSize(closureDiff.NewSize)

		var change string
		if closureDiff.NewSize > closureDiff.OldSize {
			change = "-" + formatSize(closureDiff.NewSize-closureDiff.OldSize)
		} else {
			change = "+" + formatSize(closureDiff.OldSize-closureDiff.NewSize)
		}

		fmt.Printf("  %s -> %s (%s)\n", oldSize, newSize, change)
	}

	fmt.Println()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"", "Package", "Old", "New"})
	table.SetHeaderAlignment(tablewriter.ALIGN_CENTER)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
	table.SetReflowDuringAutoWrap(false)
	table.SetBorder(false)
	table.SetRowSeparator("-")
	table.SetColumnSeparator("|")

	rows := make([][]string, 0, len(closureDiff.Diffs))
	for _, diff := range closureDiff.Diffs {
		row := make([]string, 4)

		row[0] = statusMarker(diff)
		row[1] = diff.Name
		row[2] = formatVersionList(diff.Old)
		row[3] = formatVersionList(diff.New)

		rows = append(rows, row)
	}

	table.AppendBulk(rows)

	table.Render()
	fmt.Println()
}

func formatSize(size uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	if size >= gb {
		return fmt.Sprintf("%.2f GiB", float64(size)/float64(gb))
	}
	if size >= mb {
		return fmt.Sprintf("%.2f MiB", float64(size)/float64(mb))
	}
	if size >= kb {
		return fmt.Sprintf("%.2f KiB", float64(size)/float64(kb))
	}
	return fmt.Sprintf("%d B", size)
}

func formatVersionList(versions []string) string {
	if len(versions) == 0 {
		return "∅"
	}

	out := make([]string, len(versions))
	for i, v := range versions {
		if v == "" {
			out[i] = "<unknown>"
		} else {
			out[i] = v
		}
	}

	sort.Strings(out)

	return strings.Join(out, ", ")
}

func statusMarker(diff PathDiff) string {
	switch diff.Change {
	case ChangeTypeAdd:
		return "+"
	case ChangeTypeRemove:
		return "-"
	}

	switch diff.SystemPathStatus {
	case SystemPathStatusBoth:
		return "●"
	case SystemPathStatusNewOnly:
		return "⊕"
	case SystemPathStatusOldOnly:
		return "⊖"
	default:
		return " "
	}
}
