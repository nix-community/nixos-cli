package list

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/nix-community/nixos-cli/cmd/generation/shared"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
)

var (
	// Colors
	ansiRed     = lipgloss.ANSIColor(termenv.ANSIRed)
	ansiYellow  = lipgloss.ANSIColor(termenv.ANSIYellow)
	ansiGreen   = lipgloss.ANSIColor(termenv.ANSIGreen)
	ansiWhite   = lipgloss.ANSIColor(termenv.ANSIBrightWhite)
	ansiBlue    = lipgloss.ANSIColor(termenv.ANSIBlue)
	ansiCyan    = lipgloss.ANSIColor(termenv.ANSICyan)
	ansiMagenta = lipgloss.ANSIColor(termenv.ANSIMagenta)

	// Styles
	itemStyle         = lipgloss.NewStyle().MarginLeft(4).PaddingLeft(1).Border(lipgloss.NormalBorder(), false, false, false, true)
	currentItemStyle  = lipgloss.NewStyle().MarginLeft(4).PaddingLeft(1).Foreground(ansiGreen).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(ansiGreen)
	selectedItemStyle = lipgloss.NewStyle().MarginLeft(4).PaddingLeft(1).Foreground(ansiYellow).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(ansiYellow)
	attrStyle         = lipgloss.NewStyle().Foreground(ansiCyan)
	boldStyle         = lipgloss.NewStyle().Bold(true)
	italicStyle       = lipgloss.NewStyle().Italic(true)
)

type generationItem struct {
	Generation generation.Generation
	Selected   bool
}

func (i generationItem) FilterValue() string {
	g := i.Generation
	return fmt.Sprintf("%v %v", g.Number, g.Description)
}

type generationItemDelegate struct{}

func (d generationItemDelegate) Height() int { return 7 }

func (d generationItemDelegate) Spacing() int { return 1 }

func (d generationItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d generationItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(generationItem)
	if !ok {
		return
	}

	g := i.Generation

	current := ""
	if g.IsCurrent {
		current = " (active)"
	}

	str := boldStyle.Render(fmt.Sprintf("%v%v", g.Number, current))
	if len(g.Description) > 0 {
		trimmedDesc, _, _ := strings.Cut(g.Description, "\n")
		str += italicStyle.Render(fmt.Sprintf(" - %v", trimmedDesc))
	}

	cfgRev := g.ConfigurationRevision
	if cfgRev == "" {
		cfgRev = italicStyle.Render("(unknown)")
	}

	nixpkgsRev := g.NixpkgsRevision
	if nixpkgsRev == "" {
		nixpkgsRev = italicStyle.Render("(unknown)")
	}

	kernelVersion := g.KernelVersion
	if kernelVersion == "" {
		kernelVersion = italicStyle.Render("(unknown)")
	}

	var specialisations string
	if len(g.Specialisations) > 0 {
		specialisations = strings.Join(g.Specialisations, ", ")
	} else {
		specialisations = italicStyle.Render("(none)")
	}

	str += fmt.Sprintf("\n%s    :: %s", attrStyle.Render("NixOS Version"), g.NixosVersion)
	str += fmt.Sprintf("\n%s    :: %s", attrStyle.Render("Creation Date"), g.CreationDate.Format(time.ANSIC))
	str += fmt.Sprintf("\n%s :: %s", attrStyle.Render("Nixpkgs Revision"), nixpkgsRev)
	str += fmt.Sprintf("\n%s  :: %s", attrStyle.Render("Config Revision"), cfgRev)
	str += fmt.Sprintf("\n%s   :: %s", attrStyle.Render("Kernel Version"), kernelVersion)
	str += fmt.Sprintf("\n%s  :: %s", attrStyle.Render("Specialisations"), specialisations)

	fn := itemStyle.Render

	if index == m.Index() {
		fn = func(s ...string) string {
			return currentItemStyle.Render(strings.Join(s, " "))
		}
	} else if i.Selected {
		fn = func(s ...string) string {
			return selectedItemStyle.Render(strings.Join(s, " "))
		}
	}

	_, _ = fmt.Fprint(w, fn(str))
}

type endAction interface {
	Type() string
}

type quitAction struct{}

func (a quitAction) Type() string { return "quit" }

type switchAction struct {
	Generation uint64
}

func (a switchAction) Type() string { return "switch" }

type deleteAction struct {
	Generations []uint64
}

func (a deleteAction) Type() string { return "delete" }

type model struct {
	list    list.Model
	profile string
	action  endAction
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 1)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.action = quitAction{}
			return m, tea.Quit

		case "enter":
			g := m.list.SelectedItem().(generationItem).Generation
			m.action = switchAction{Generation: g.Number}
			return m, tea.Quit

		case "d":
			items := m.list.Items()
			gens := make([]uint64, 0, len(items))
			for _, v := range items {
				i := v.(generationItem)
				if i.Selected {
					gens = append(gens, i.Generation.Number)
				}
			}

			if len(gens) > 0 {
				m.action = deleteAction{Generations: gens}
				return m, tea.Quit
			}

		case tea.KeySpace.String():
			i := m.list.SelectedItem().(generationItem)
			if !i.Generation.IsCurrent {
				i.Selected = !i.Selected
				m.list.SetItem(m.list.Index(), i)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

const (
	CLEAR       = "\x1B[2J"
	MV_TOP_LEFT = "\x1B[H"
)

func clearScreen() {
	fmt.Print(CLEAR + MV_TOP_LEFT)
}

func runGenerationSwitchCmd(log logger.Logger, generation uint64, profile string) error {
	argv := []string{os.Args[0], "generation", "-p", profile, "switch", fmt.Sprintf("%v", generation)}

	cmd := exec.Command(argv[0], argv[1:]...)

	log.CmdArray(argv)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func runGenerationDeleteCmd(log logger.Logger, generations []uint64, profile string) error {
	argv := []string{os.Args[0], "generation", "-p", profile, "delete"}
	for _, v := range generations {
		argv = append(argv, fmt.Sprintf("%v", v))
	}

	cmd := exec.Command(argv[0], argv[1:]...)

	log.CmdArray(argv)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (m model) View() string {
	// Clear the view before exiting.
	if m.action != nil {
		return ""
	}

	return "\n" + m.list.View()
}

func newGenerationList(generations []generation.Generation) list.Model {
	items := make([]list.Item, len(generations))
	for i, v := range generations {
		items[i] = generationItem{
			Generation: v,
			Selected:   false,
		}
	}

	l := list.New(items, generationItemDelegate{}, 0, 0)

	l.Title = "NixOS Generations"

	l.Styles.Title = lipgloss.NewStyle().MarginLeft(2).Background(ansiRed).Foreground(ansiWhite)
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	l.Styles.StatusBar = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1).Foreground(ansiMagenta)

	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(ansiBlue).Bold(true).PaddingLeft(2)
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(ansiBlue)
	l.FilterInput.Cursor.Style = lipgloss.NewStyle().Foreground(ansiBlue)
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(ansiBlue)
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle().Foreground(ansiBlue)

	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "select for deletion"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "switch to generation"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete selected generations"),
			),
		}
	}

	return l
}

func generationUI(log logger.Logger, profile string, generations []generation.Generation) error {
	closeLogFile, _ := cmdUtils.ConfigureBubbleTeaLogger("genlist")
	defer closeLogFile()

	l := newGenerationList(generations)

	m := model{
		list:    l,
		profile: profile,
	}

	for {
		finalM, err := tea.NewProgram(&m).Run()
		if err != nil {
			return err
		}

		action := finalM.(model).action

		switch a := action.(type) {
		case quitAction:
			return nil
		case switchAction:
			err = runGenerationSwitchCmd(log, a.Generation, profile)
		case deleteAction:
			err = runGenerationDeleteCmd(log, a.Generations, profile)
		}

		if err != nil {
			log.Errorf("%v", err)
		}

		log.Info("returning to main window")
		if err != nil {
			time.Sleep(time.Second * 3)
		} else {
			time.Sleep(time.Second)
		}

		reloadedGenerations, err := genUtils.LoadGenerations(log, profile, true)
		if err != nil {
			return err
		}
		m.list = newGenerationList(reloadedGenerations)
		m.action = nil
		clearScreen()
	}
}
