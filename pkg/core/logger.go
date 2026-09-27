package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// ANSI color codes
const (
	ColorBold       = "\033[1m"
	ColorRed        = "\033[31m"
	ColorBoldRed    = "\033[1;31m"
	ColorYellow     = "\033[33m"
	ColorBoldYellow = "\033[1;33m"
	ColorGreen      = "\033[32m"
	ColorBoldGreen  = "\033[1;32m"
	ColorBlue       = "\033[34m"
	ColorBoldBlue   = "\033[1;34m"
	ColorCyan       = "\033[36m"
	ColorBoldCyan   = "\033[1;36m"
	ColorGrayBold   = "\033[90m"
	ColorReset      = "\033[0m"
	Bold            = "\033[1m"
)

// Logger provides structured logging with colors
type Logger struct {
	logFile *os.File
}

// NewLogger creates a new Logger instance
func NewLogger(logPath string) (*Logger, error) {
	var logFile *os.File
	var err error

	if logPath != "" {
		logFile, err = OpenLogFile(logPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
	}

	return &Logger{logFile: logFile}, nil
}

// OpenLogFile confines logs to a private directory and refuses symlink files.
func OpenLogFile(logPath string) (*os.File, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe log directory: %s", dir)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(logPath, syscall.O_APPEND|syscall.O_CREAT|syscall.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), logPath), nil
}

// Close closes the log file
func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.log("info", msg, ColorReset)
}

// Warn logs a warning message
// Record writes msg to the log file only.
func (l *Logger) Record(msg string) {
	if l.logFile != nil && msg != "" {
		fmt.Fprintf(l.logFile, "%s %-5s %s\n", time.Now().Format("2006-01-02 15:04:05"), "INFO", stripANSI(msg))
	}
}

func (l *Logger) Warn(msg string) {
	l.log("warn", msg, ColorYellow)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.log("error", msg, ColorBoldRed)
}

// Command logs a command message
func (l *Logger) Command(msg string) {
	l.log("cmd", msg, ColorBlue)
}

func (l *Logger) log(level, msg, color string) {
	if msg == "" {
		return
	}
	// Terminal: a coloured glyph per level. "[OK] " messages become a green ✔.
	glyph := ColorBoldBlue + "•" + ColorReset
	text := msg
	switch level {
	case "error":
		glyph = ColorBoldRed + "✖" + ColorReset
	case "warn":
		glyph = ColorBoldYellow + "▲" + ColorReset
	case "cmd":
		glyph = ColorGrayBold + "$" + ColorReset
	case "info":
		if rest, ok := strings.CutPrefix(msg, "[OK] "); ok {
			glyph = ColorBoldGreen + "✔" + ColorReset
			text = rest
		}
	}
	lines := splitLines(text, 110)

	termMu.Lock()
	clearLineLocked()
	for i, line := range lines {
		lead := "  " + glyph + " "
		if i > 0 {
			lead = "    "
		}
		fmt.Println(lead + color + line + ColorReset)
	}
	termMu.Unlock()

	// Log file: plain text with a timestamp and level, no colour codes.
	if l.logFile != nil {
		fmt.Fprintf(l.logFile, "%s %-5s %s\n", time.Now().Format("2006-01-02 15:04:05"), strings.ToUpper(level), stripANSI(msg))
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// Colorize applies a color code to a string
func Colorize(text, color string) string {
	return color + text + ColorReset
}

// ColorizeSubstr applies a color to a specific substring within a larger string
func ColorizeSubstr(s, substring, color, restoreColor string) string {
	if substring == "" {
		return s
	}

	result := ""
	remaining := s

	for {
		idx := strings.Index(remaining, substring)
		if idx == -1 {
			result += remaining
			break
		}

		result += remaining[:idx]
		result += color + substring + ColorReset + restoreColor
		remaining = remaining[idx+len(substring):]
	}

	return result
}

func splitLines(s string, maxLen int) []string {
	if len(s) <= maxLen {
		return []string{s}
	}

	var lines []string
	start := 0
	for start < len(s) {
		end := start + maxLen
		if end >= len(s) {
			lines = append(lines, s[start:])
			break
		}
		// Try to break at space
		spaceIdx := -1
		for i := end - 1; i > start; i-- {
			if s[i] == ' ' || s[i] == '\t' {
				spaceIdx = i
				break
			}
		}
		if spaceIdx == -1 {
			spaceIdx = end - 3
			lines = append(lines, s[start:spaceIdx]+"...")
			start = spaceIdx + 3
		} else {
			lines = append(lines, s[start:spaceIdx])
			start = spaceIdx + 1
		}
	}
	return lines
}
