package logview

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const recentEventLimit = 5

type dashboardState struct {
	startedAt    time.Time
	total        int
	statusCounts map[string]int
	durations    []float64
	recent       []dashboardEvent
	lastSeenAt   time.Time
}

func newDashboardState(now time.Time) *dashboardState {
	return &dashboardState{
		startedAt: now,
		statusCounts: map[string]int{
			"2xx": 0,
			"3xx": 0,
			"4xx": 0,
			"5xx": 0,
		},
		durations: make([]float64, 0, recentEventLimit*100),
		recent:    make([]dashboardEvent, 0, recentEventLimit),
	}
}

func (s *dashboardState) addEvent(evt AccessEvent) {
	class := statusClass(evt.Status)
	s.total++
	s.statusCounts[class]++
	s.lastSeenAt = eventTimestamp(evt)
	s.durations = append(s.durations, evt.Duration)

	event := dashboardEvent{
		at:     eventTimestamp(evt),
		method: evt.Request.Method,
		uri:    evt.Request.URI,
		status: evt.Status,
	}

	s.recent = append([]dashboardEvent{event}, s.recent...)
	if len(s.recent) > recentEventLimit {
		s.recent = s.recent[:recentEventLimit]
	}
}

func (s *dashboardState) frame(now time.Time, opts dashboardFrameOptions, width int) string {
	uptime := now.Sub(s.startedAt).Round(time.Second)
	if uptime < 0 {
		uptime = 0
	}
	currentOpen := 0
	if !s.lastSeenAt.IsZero() && now.Sub(s.lastSeenAt) <= 5*time.Second {
		currentOpen = 1
	}
	title := "serve (Ctrl+C to quit)"
	if width > 0 {
		title = clampLine(title, width)
	}
	panel := []string{
		title,
		"",
		"Session Status                online",
		fmt.Sprintf("Listen                        %s", dashboardListenValue(opts.listen)),
		fmt.Sprintf("Root                          %s", opts.root),
		fmt.Sprintf("Uptime                        %s", uptime),
		fmt.Sprintf("Mode                          %s", opts.mode),
		fmt.Sprintf("Forwarding                    %s", forwardingValue(opts.upstream)),
		"",
		"Connections                   ttl     opn     p50     p90",
		fmt.Sprintf("                              %-7d %-7d %-7s %-7s", s.total, currentOpen, percentileLabel(s.durations, 0.50), percentileLabel(s.durations, 0.90)),
		"",
		"HTTP Requests",
		"-------------",
		fmt.Sprintf(
			"Status Buckets: %s %s %s %s",
			colorizeClassCount("2xx", s.statusCounts["2xx"], opts.colorize),
			colorizeClassCount("3xx", s.statusCounts["3xx"], opts.colorize),
			colorizeClassCount("4xx", s.statusCounts["4xx"], opts.colorize),
			colorizeClassCount("5xx", s.statusCounts["5xx"], opts.colorize),
		),
	}
	if len(s.recent) == 0 {
		panel = append(panel, "waiting for requests")
	} else {
		for _, evt := range s.recent {
			panel = append(panel, formatDashboardEvent(evt, width))
		}
	}
	return strings.Join(panel, "\n")
}

type dashboardFrameOptions struct {
	colorize bool
	listen   string
	root     string
	upstream string
	mode     string
}

type dashboardEvent struct {
	at     time.Time
	method string
	uri    string
	status int
}

func formatDashboardEvent(evt dashboardEvent, width int) string {
	statusText := http.StatusText(evt.status)
	if statusText == "" {
		statusText = "-"
	}
	ts := evt.at.Format("15:04:05.000 MST")
	pathWidth := 30
	if width > 0 {
		// time + spaces + method + status + text ~= 36 chars
		pathWidth = maxInt(12, width-36)
	}
	return fmt.Sprintf("%s %-6s %-*s %3d %s", ts, evt.method, pathWidth, trimPath(evt.uri, pathWidth), evt.status, statusText)
}

func percentileLabel(values []float64, percentile float64) string {
	if len(values) == 0 {
		return "-"
	}
	cpy := append([]float64(nil), values...)
	sort.Float64s(cpy)
	idx := int(percentile * float64(len(cpy)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cpy) {
		idx = len(cpy) - 1
	}
	return humanDuration(cpy[idx])
}

func forwardingValue(upstream string) string {
	if upstream == "" {
		return "static file server"
	}
	return upstream
}

func dashboardListenValue(listen string) string {
	if strings.HasPrefix(listen, ":") {
		return "http://localhost" + listen
	}
	return "http://" + listen
}

func colorizeClassCount(class string, count int, colorize bool) string {
	label := fmt.Sprintf("%s=%d", class, count)
	if !colorize {
		return label
	}
	var color lipgloss.Color
	switch class {
	case "2xx":
		color = lipgloss.Color("42")
	case "3xx":
		color = lipgloss.Color("44")
	case "4xx":
		color = lipgloss.Color("214")
	case "5xx":
		color = lipgloss.Color("196")
	default:
		return label
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)
}

func supportsInteractiveDashboard() bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func supportsColorOutput() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

func eventTimestamp(evt AccessEvent) time.Time {
	if !evt.Timestamp.IsZero() {
		return evt.Timestamp.Local()
	}
	return time.Now()
}

type dashboardTickMsg time.Time
type dashboardEventMsg struct {
	event AccessEvent
}

type dashboardErrMsg struct {
	err error
}

type dashboardModel struct {
	opts  dashboardFrameOptions
	state *dashboardState
	now   time.Time
	width int
	err   error
}

func newDashboardModel(opts dashboardFrameOptions) dashboardModel {
	now := time.Now()
	return dashboardModel{
		opts:  opts,
		state: newDashboardState(now),
		now:   now,
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return dashboardTickCmd()
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case dashboardTickMsg:
		m.now = time.Time(v)
		return m, dashboardTickCmd()
	case dashboardEventMsg:
		m.state.addEvent(v.event)
		return m, nil
	case tea.WindowSizeMsg:
		m.width = v.Width
		return m, nil
	case dashboardErrMsg:
		m.err = v.err
		return m, tea.Quit
	case tea.KeyMsg:
		if v.String() == "ctrl+c" || v.String() == "q" {
			return m, tea.Quit
		}
	case tea.QuitMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("dashboard error: %v", m.err)
	}
	frame := m.state.frame(m.now, m.opts, m.width)
	if m.width > 0 {
		lines := strings.Split(frame, "\n")
		for i := range lines {
			lines[i] = clampLine(lines[i], m.width)
		}
		frame = strings.Join(lines, "\n")
	}
	if m.opts.colorize {
		frame = styleDashboardFrame(frame)
	}
	return frame
}

func styleDashboardFrame(frame string) string {
	lines := strings.Split(frame, "\n")
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	subtleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	for i, line := range lines {
		switch {
		case i == 0:
			lines[i] = titleStyle.Render(line)
		case line == "HTTP Requests" || line == "Session Status                online" || strings.HasPrefix(line, "Connections"):
			lines[i] = sectionStyle.Render(line)
		case line == "-------------":
			lines[i] = subtleStyle.Render(line)
		case strings.HasPrefix(line, "waiting for requests"):
			lines[i] = subtleStyle.Italic(true).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func dashboardTickCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return dashboardTickMsg(t)
	})
}

func runDashboardUI(ctx context.Context, opts dashboardFrameOptions, events <-chan AccessEvent, errs <-chan error) error {
	model := newDashboardModel(opts)
	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		for {
			select {
			case <-ctx.Done():
				p.Send(tea.Quit())
				return
			case evt, ok := <-events:
				if !ok {
					p.Send(tea.Quit())
					return
				}
				p.Send(dashboardEventMsg{event: evt})
			case err, ok := <-errs:
				if ok && err != nil {
					p.Send(dashboardErrMsg{err: err})
					return
				}
			}
		}
	}()

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("run dashboard ui: %w", err)
	}
	if dm, ok := finalModel.(dashboardModel); ok && dm.err != nil {
		return dm.err
	}
	return nil
}

func clampLine(line string, max int) string {
	if len(line) <= max {
		return line
	}
	if max <= 3 {
		return line[:max]
	}
	return line[:max-3] + "..."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
