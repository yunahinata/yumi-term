package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Palette ──────────────────────────────────────────────────────────────────
var (
	crust   = lipgloss.Color("#11111b")
	mantle  = lipgloss.Color("#181825")
	overlay = lipgloss.Color("#313244")
	muted   = lipgloss.Color("#585b70")
	text    = lipgloss.Color("#cdd6f4")
	green   = lipgloss.Color("#a6e3a1")
	red     = lipgloss.Color("#f38ba8")
	blue    = lipgloss.Color("#89b4fa")
	mauve   = lipgloss.Color("#cba6f7")
	peach   = lipgloss.Color("#fab387")
)

// ── Model ────────────────────────────────────────────────────────────────────

type msgKind int

const (
	kindUser msgKind = iota
	kindSystem
	kindInfo
)

type message struct {
	kind     msgKind
	text     string
	exitCode int
	ts       time.Time
}

// Тип сообщения для асинхронного возврата результатов команды
type commandResultMsg struct {
	stdout   string
	stderr   string
	exitCode int
	newCwd   string
}

type model struct {
	history    []message
	textInput  textinput.Model
	width      int
	height     int
	scrollOff  int    // lines scrolled up from bottom (0 = pinned to bottom)
	currentDir string // Отслеживание текущей рабочей директории
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "type a shell command…"
	ti.Focus()
	ti.CharLimit = 512
	ti.Prompt = ""

	dir, _ := os.Getwd()

	return model{
		history: []message{{
			kind: kindInfo,
			text: "welcome to yumi term — type any shell command, scroll with mouse wheel or ↑↓",
			ts:   time.Now(),
		}},
		textInput:  ti,
		currentDir: dir,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// Асинзовая функция для запуска процессов в фоне и вычисления нового пути cd
func runCommandAsync(input string, currentDir string) tea.Cmd {
	return func() tea.Msg {
		var stdout, stderr bytes.Buffer

		// Обертка: переходим в текущую директорию, выполняем команду, сохраняем её статус,
		// выводим pwd для отслеживания изменений cd, и возвращаем исходный статус.
		wrapper := fmt.Sprintf("cd %q && (%s) ; GoExitStatus=$? ; pwd ; exit $GoExitStatus", currentDir, input)

		c := exec.Command("sh", "-c", wrapper)
		c.Stdout = &stdout
		c.Stderr = &stderr
		err := c.Run()

		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = 1
			}
		}

		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		// Парсим новую директорию из последней строчки stdout
		stdoutLines := strings.Split(strings.TrimRight(stdoutStr, "\n"), "\n")
		newCwd := currentDir
		actualStdout := ""

		if len(stdoutLines) > 0 {
			newCwd = stdoutLines[len(stdoutLines)-1]
			actualStdout = strings.Join(stdoutLines[:len(stdoutLines)-1], "\n")
		}

		return commandResultMsg{
			stdout:   actualStdout,
			stderr:   stderrStr,
			exitCode: exitCode,
			newCwd:   newCwd,
		}
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.textInput.Width = msg.Width - 18

		case tea.MouseMsg:
			if msg.Type == tea.MouseWheelUp {
				m.scrollOff += 3
			}
			if msg.Type == tea.MouseWheelDown {
				if m.scrollOff > 3 {
					m.scrollOff -= 3
				} else {
					m.scrollOff = 0
				}
			}
			return m, nil

		case tea.KeyMsg:
			switch msg.Type {
				case tea.KeyCtrlC, tea.KeyEsc:
					return m, tea.Quit

				case tea.KeyUp:
					m.scrollOff += 3
					return m, nil

				case tea.KeyDown:
					if m.scrollOff > 3 {
						m.scrollOff -= 3
					} else {
						m.scrollOff = 0
					}
					return m, nil

				case tea.KeyEnter:
					input := strings.TrimSpace(m.textInput.Value())
					if input == "" {
						return m, nil
					}

					// Добавляем команду юзера на экран
					m.history = append(m.history, message{
						kind: kindUser,
						text: input,
						ts:   time.Now(),
					})

					m.textInput.SetValue("")
					m.scrollOff = 0 // Сбрасываем скролл вниз

					// Запускаем фоновую задачу и передаем туда текущую директорию
					return m, runCommandAsync(input, m.currentDir)
			}

			// Сюда прилетает ответ от завершившейся фоновой команды
				case commandResultMsg:
					m.currentDir = msg.newCwd // Обновляем путь приложения

					out := strings.TrimRight(msg.stdout, "\n")
					if out == "" {
						out = strings.TrimRight(msg.stderr, "\n")
					}
					if out == "" {
						if msg.exitCode == 0 {
							out = "✓  done"
						} else {
							out = fmt.Sprintf("exited with code %d", msg.exitCode)
						}
					}

					m.history = append(m.history, message{
						kind:     kindSystem,
						text:     out,
						exitCode: msg.exitCode,
						ts:       time.Now(),
					})
					return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func wrapText(s string, maxW int) []string {
	if maxW <= 0 {
		return []string{s}
	}
	var out []string
	for _, raw := range strings.Split(s, "\n") {
		for utf8.RuneCountInString(raw) > maxW {
			cut := 0
			i := 0
			for _, _ = range raw {
				if i >= maxW {
					break
				}
				cut = i
				i++
			}
			if cut == 0 {
				cut = maxW
			}
			out = append(out, raw[:cut])
			raw = raw[cut:]
		}
		out = append(out, raw)
	}
	return out
}

func renderBubble(msg message, maxW int) string {
	ts := msg.ts.Format("15:04")

	switch msg.kind {

		case kindInfo:
			return lipgloss.NewStyle().
			Foreground(muted).
			Italic(true).
			PaddingLeft(2).
			Render(msg.text)

		case kindUser:
			maxContent := maxW*2/3 - 4
			wrapped := wrapText(msg.text, maxContent)

			bubble := lipgloss.NewStyle().
			Foreground(crust).
			Background(blue).
			Padding(0, 2).
			Bold(true)

			tsStyle := lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

			var lines []string
			for i, line := range wrapped {
				var content string
				if i == 0 {
					content = lipgloss.JoinHorizontal(lipgloss.Center,
									  tsStyle.Render(ts+"  "),
									  bubble.Render(line),
					)
				} else {
					content = lipgloss.PlaceHorizontal(maxW, lipgloss.Right, bubble.Render("  "+line))
				}
				lines = append(lines, lipgloss.PlaceHorizontal(maxW, lipgloss.Right, content))
			}
			return strings.Join(lines, "\n")

		case kindSystem:
			maxContent := maxW*2/3 - 4
			wrapped := wrapText(msg.text, maxContent)

			accent := green
			if msg.exitCode != 0 {
				accent = red
			}

			bubble := lipgloss.NewStyle().
			Foreground(text).
			Background(overlay).
			Padding(0, 2).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(accent)

			tsStyle := lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

			var lines []string
			for i, line := range wrapped {
				b := bubble.Render(line)
				if i == 0 {
					lines = append(lines, b+"  "+tsStyle.Render(ts))
				} else {
					lines = append(lines, b)
				}
			}
			return strings.Join(lines, "\n")
	}
	return msg.text
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	var allLines []string
	for i, msg := range m.history {
		if i > 0 {
			allLines = append(allLines, "")
		}
		allLines = append(allLines, strings.Split(renderBubble(msg, m.width-2), "\n")...)
	}

	reserved := 3
	histH := m.height - reserved
	if histH < 1 {
		histH = 1
	}

	maxScroll := len(allLines) - histH
	if maxScroll < 0 {
		maxScroll = 0
	}
	off := m.scrollOff
	if off > maxScroll {
		off = maxScroll
	}

	start := len(allLines) - histH - off
	if start < 0 {
		start = 0
	}
	end := start + histH
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := allLines[start:end]

	for len(visible) < histH {
		visible = append([]string{""}, visible...)
	}

	var sb strings.Builder

	// ── header ───────────────────────────────────────────────────────────────
	title := lipgloss.NewStyle().Foreground(mauve).Bold(true).Render("yumi term")
	hint := lipgloss.NewStyle().Foreground(muted).Render("  shell")
	scrollBadge := ""
	if off > 0 {
		scrollBadge = lipgloss.NewStyle().Foreground(peach).Render(fmt.Sprintf("  ↑ %d", off))
	}
	header := lipgloss.NewStyle().
	Background(mantle).
	Width(m.width).
	Padding(0, 2).
	Render(title + hint + scrollBadge)
	sb.WriteString(header + "\n")

	// ── history ───────────────────────────────────────────────────────────────
	for _, line := range visible {
		sb.WriteString(line + "\n")
	}

	// ── divider ───────────────────────────────────────────────────────────────
	sb.WriteString(lipgloss.NewStyle().Foreground(overlay).Render(strings.Repeat("─", m.width)) + "\n")

	// ── input ─────────────────────────────────────────────────────────────────
	home, _ := os.UserHomeDir()
	cwd := m.currentDir
	cwd = strings.Replace(cwd, home, "~", 1)

	prompt := lipgloss.NewStyle().Foreground(mauve).Bold(true).Render("❯ ")
	cwdPart := lipgloss.NewStyle().Foreground(peach).Render(cwd + "  ")

	inputLine := lipgloss.NewStyle().
	Background(crust).
	Width(m.width).
	Padding(0, 1).
	Render(lipgloss.JoinHorizontal(lipgloss.Center, prompt, cwdPart, m.textInput.View()))
	sb.WriteString(inputLine)

	return sb.String()
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	p := tea.NewProgram(
		initialModel(),
			    tea.WithAltScreen(),
			    tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
