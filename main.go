// zap — a TUI app launcher for Windows.
// https://github.com/AlexandrosLiaskos/zap
package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/windows/registry"
)

// ── Styles ──────────────────────────────────────────────────────

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	searchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
)

// ── Data ────────────────────────────────────────────────────────

type appEntry struct {
	Name  string
	AppID string // shell:AppsFolder ID (from Get-StartApps)
	Path  string // install location (from registry)
}

// ── TUI State ───────────────────────────────────────────────────

type viewMode int

const (
	modeApps viewMode = iota
	modeSearch
)

type model struct {
	input    textinput.Model
	allApps  []appEntry
	filtered []appEntry
	cursor   int
	offset   int
	maxShow  int
	width    int
	height   int
	mode     viewMode
	launched string
	quitting bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "search apps · / to search web · esc to quit"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	apps := loadApps()

	return model{
		input:    ti,
		allApps:  apps,
		filtered: apps,
		maxShow:  15,
	}
}

// ── Load Apps ───────────────────────────────────────────────────

// ghostApps lists display names (lowercase) of apps that have been uninstalled
// but still appear in Get-StartApps due to Windows caching.
var ghostApps = map[string]bool{
	"google chrome": true,
}

// loadApps collects apps from two sources:
//  1. Start menu entries via Get-StartApps (PowerShell)
//  2. Registry uninstall entries with an InstallLocation
//
// Ghost entries are excluded. Results are sorted alphabetically.
func loadApps() []appEntry {
	seen := make(map[string]bool)
	var apps []appEntry

	// Start menu apps
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-StartApps | ForEach-Object { "$($_.Name)|$($_.AppID)" }`).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
			if len(parts) != 2 || parts[0] == "" {
				continue
			}
			lower := strings.ToLower(parts[0])
			if seen[lower] || ghostApps[lower] {
				continue
			}
			seen[lower] = true
			apps = append(apps, appEntry{Name: parts[0], AppID: parts[1]})
		}
	}

	// Registry apps (uninstall entries with exe paths)
	regPaths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, rp := range regPaths {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, rp, registry.READ)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, name := range names {
			sub, err := registry.OpenKey(registry.LOCAL_MACHINE, rp+`\`+name, registry.READ)
			if err != nil {
				continue
			}
			displayName, _, _ := sub.GetStringValue("DisplayName")
			installLoc, _, _ := sub.GetStringValue("InstallLocation")
			sub.Close()
			if displayName == "" || installLoc == "" {
				continue
			}
			lower := strings.ToLower(displayName)
			if seen[lower] {
				continue
			}
			seen[lower] = true
			apps = append(apps, appEntry{Name: displayName, Path: installLoc})
		}
	}

	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps
}

// ── Bubble Tea ──────────────────────────────────────────────────

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.maxShow = max(m.height-6, 5)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleEnter()
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxShow {
					m.offset = m.cursor - m.maxShow + 1
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	query := m.input.Value()
	if strings.HasPrefix(query, "/") {
		m.mode = modeSearch
	} else {
		m.mode = modeApps
		m.filtered = filterApps(m.allApps, query)
		if m.cursor >= len(m.filtered) {
			m.cursor = max(len(m.filtered)-1, 0)
		}
		m.offset = 0
	}

	return m, cmd
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.mode == modeSearch {
		query := strings.TrimSpace(strings.TrimPrefix(m.input.Value(), "/"))
		if query != "" {
			searchURL := "https://duckduckgo.com/?q=" + url.QueryEscape(query)
			chromium := os.Getenv("LOCALAPPDATA") + `\Chromium\Application\chrome.exe`
			_ = exec.Command(chromium, searchURL).Start()
			m.launched = "Searching: " + query
		}
		m.quitting = true
		return m, tea.Quit
	}

	if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
		app := m.filtered[m.cursor]
		m.launched = app.Name
		if app.AppID != "" {
			_ = exec.Command("explorer.exe", "shell:AppsFolder\\"+app.AppID).Start()
		} else if app.Path != "" {
			_ = exec.Command("explorer.exe", app.Path).Start()
		}
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		if m.launched != "" {
			return dimStyle.Render("  → "+m.launched) + "\n"
		}
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("⚡ zap") + "\n")
	b.WriteString("  " + m.input.View() + "\n\n")

	if m.mode == modeSearch {
		query := strings.TrimSpace(strings.TrimPrefix(m.input.Value(), "/"))
		b.WriteString(searchStyle.Render("  🔍 Search: ") + normalStyle.Render(query) + "\n")
		b.WriteString(dimStyle.Render("  enter to search DuckDuckGo") + "\n")
		return b.String()
	}

	end := min(m.offset+m.maxShow, len(m.filtered))
	if m.offset > 0 {
		b.WriteString(dimStyle.Render("  ↑ more") + "\n")
	}
	for i := m.offset; i < end; i++ {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("  ▸ "+m.filtered[i].Name) + "\n")
		} else {
			b.WriteString(normalStyle.Render("    "+m.filtered[i].Name) + "\n")
		}
	}
	if end < len(m.filtered) {
		b.WriteString(dimStyle.Render("  ↓ more") + "\n")
	}
	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("  no matches") + "\n")
	}

	b.WriteString("\n" + helpStyle.Render(fmt.Sprintf("  %d apps · ↑↓ navigate · enter launch · / search · esc quit", len(m.filtered))))
	return b.String()
}

// ── Helpers ─────────────────────────────────────────────────────

func filterApps(apps []appEntry, query string) []appEntry {
	if query == "" {
		return apps
	}
	q := strings.ToLower(query)
	words := strings.Fields(q)
	var out []appEntry
	for _, app := range apps {
		hay := strings.ToLower(app.Name)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, app)
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Entry Point ─────────────────────────────────────────────────

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "zap: %v\n", err)
		os.Exit(1)
	}
}
