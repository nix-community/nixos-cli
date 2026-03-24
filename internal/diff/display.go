package diff

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

func displayDiffResults(closureDiff *ClosureDiff) {
	if len(closureDiff.Diffs) == 0 {
		return
	}

	fmt.Println("<<<", color.RedString(closureDiff.Old.Path))
	fmt.Println(">>>", color.GreenString(closureDiff.New.Path))

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
	if added > 0 {
		fmt.Println(color.GreenString("  + %d added", added))
	}
	if removed > 0 {
		fmt.Println(color.RedString("  - %d removed", removed))
	}
	if changed > 0 {
		fmt.Println(color.YellowString("  ~ %d changed", changed))
	}

	fmt.Println("\nSize:")
	if closureDiff.Old.Size == closureDiff.New.Size {
		fmt.Println("  (no change)")
	} else {
		oldSize := formatSize(closureDiff.Old.Size)
		newSize := formatSize(closureDiff.New.Size)

		var change string
		if closureDiff.New.Size > closureDiff.Old.Size {
			change = color.RedString("+%s", formatSize(closureDiff.New.Size-closureDiff.Old.Size))
		} else {
			change = color.GreenString("-%s", formatSize(closureDiff.Old.Size-closureDiff.New.Size))
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

		var colorString func(string, ...any) string
		switch diff.Change {
		case ChangeTypeAdd:
			colorString = color.GreenString
		case ChangeTypeRemove:
			colorString = color.RedString
		case ChangeTypeChange:
			colorString = color.YellowString
		default:
			colorString = func(v string, a ...any) string { return v }
		}

		colorVersionList := func(value string) string {
			if value == nullSymbol {
				return nullSymbol
			}
			return colorString(value)
		}

		row[0] = colorString(statusMarker(diff))
		row[1] = colorString(diff.Name)
		row[2] = colorVersionList(formatVersionList(diff.Old))
		row[3] = colorVersionList(formatVersionList(diff.New))

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

var nullSymbol = "∅"

func formatVersionList(versions []string) string {
	if len(versions) == 0 {
		return nullSymbol
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
