package core

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Terminal presentation: spinners for long steps, in-place progress bars,
// step headers and the logo banner. Everything degrades to plain, one line
// per event output when stdout isn't a terminal, TERM is dumb or NO_COLOR is
// set, so logs and pipes stay readable.

var (
	termMu     sync.Mutex
	activeTask *Task
	fancyOnce  sync.Once
	fancy      bool
)

// Fancy reports whether stdout is an interactive, colour-capable terminal.
func Fancy() bool {
	fancyOnce.Do(func() {
		info, err := os.Stdout.Stat()
		fancy = err == nil && info.Mode()&os.ModeCharDevice != 0 &&
			os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
	})
	return fancy
}

// trueColor reports whether the terminal advertises 24-bit colour.
func trueColor() bool {
	c := strings.ToLower(os.Getenv("COLORTERM"))
	return c == "truecolor" || c == "24bit"
}

// clearLineLocked erases a spinner or progress line before other output.
// termMu must be held.
func clearLineLocked() {
	if Fancy() && activeTask != nil {
		fmt.Print("\r\033[2K")
	}
}

// --- Spinner tasks ----------------------------------------------------------

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Task is a long-running step shown as a spinner with elapsed time, then as a
// ✔ or ✖ line.
type Task struct {
	label string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

// StartTask begins a spinner for label. Call Done when the step finishes.
func StartTask(label string) *Task {
	t := &Task{label: label, start: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	if !Fancy() {
		fmt.Printf("  … %s\n", label)
		close(t.done)
		return t
	}
	termMu.Lock()
	activeTask = t
	termMu.Unlock()
	go t.spin()
	return t
}

func (t *Task) spin() {
	defer close(t.done)
	tick := time.NewTicker(90 * time.Millisecond)
	defer tick.Stop()
	for i := 0; ; i++ {
		termMu.Lock()
		fmt.Printf("\r\033[2K  %s%s%s %s %s%s%s", ColorBoldCyan, spinnerFrames[i%len(spinnerFrames)], ColorReset, t.label, ColorGrayBold, elapsed(time.Since(t.start)), ColorReset)
		termMu.Unlock()
		select {
		case <-t.stop:
			return
		case <-tick.C:
		}
	}
}

// Done ends the task and prints its outcome.
func (t *Task) Done(err error) {
	if Fancy() {
		close(t.stop)
		<-t.done
	}
	termMu.Lock()
	defer termMu.Unlock()
	if activeTask == t {
		fmt.Print("\r\033[2K")
		activeTask = nil
	}
	took := elapsed(time.Since(t.start))
	if err != nil {
		fmt.Printf("  %s✖%s %s %s%s%s\n", ColorBoldRed, ColorReset, t.label, ColorGrayBold, took, ColorReset)
		return
	}
	fmt.Printf("  %s✔%s %s %s%s%s\n", ColorBoldGreen, ColorReset, t.label, ColorGrayBold, took, ColorReset)
}

func elapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// --- Progress bars --------------------------------------------------------------

// Progress draws an in-place download bar. Without a terminal it prints a line
// every 25%.
type Progress struct {
	label       string
	total, done int64
	start, last time.Time
	lastPct     int
}

// NewProgress starts a bar for total bytes (0 if unknown).
func NewProgress(label string, total int64) *Progress {
	p := &Progress{label: label, total: total, start: time.Now()}
	termMu.Lock()
	activeTask = &Task{}
	termMu.Unlock()
	return p
}

// Done returns the bytes recorded so far.
func (p *Progress) Done() int64 { return p.done }

// Add records n more bytes and redraws at most ten times a second.
func (p *Progress) Add(n int64) {
	p.done += n
	now := time.Now()
	if Fancy() {
		if now.Sub(p.last) < 100*time.Millisecond && p.done != p.total {
			return
		}
		p.last = now
		termMu.Lock()
		fmt.Print("\r\033[2K  " + p.render(now))
		termMu.Unlock()
		return
	}
	if p.total > 0 {
		if pct := int(p.done * 100 / p.total); pct >= p.lastPct+25 {
			p.lastPct = pct - pct%25
			fmt.Printf("  … %s: %d%%\n", p.label, p.lastPct)
		}
	}
}

func (p *Progress) render(now time.Time) string {
	const width = 28
	rate := ""
	if secs := now.Sub(p.start).Seconds(); secs >= 1 {
		rate = fmt.Sprintf("  %.1f MB/s", float64(p.done)/secs/(1<<20))
	}
	if p.total <= 0 {
		return fmt.Sprintf("%s%s%s %s  %.0f MB%s", ColorBoldCyan, spinnerFrames[int(now.UnixMilli()/90)%len(spinnerFrames)], ColorReset, p.label, float64(p.done)/(1<<20), rate)
	}
	frac := float64(p.done) / float64(p.total)
	filled := int(frac * width)
	bar := ColorBoldCyan + strings.Repeat("█", filled) + ColorGrayBold + strings.Repeat("░", width-filled) + ColorReset
	return fmt.Sprintf("%s %s %3.0f%%  %s%.0f/%.0f MB%s%s", p.label, bar, frac*100, ColorGrayBold, float64(p.done)/(1<<20), float64(p.total)/(1<<20), rate, ColorReset)
}

// Finish replaces the bar with a ✔ or ✖ line.
func (p *Progress) Finish(err error) {
	termMu.Lock()
	defer termMu.Unlock()
	activeTask = nil
	if Fancy() {
		fmt.Print("\r\033[2K")
	}
	size := fmt.Sprintf("%.0f MB", float64(p.done)/(1<<20))
	if err != nil {
		fmt.Printf("  %s✖%s %s %s%s%s\n", ColorBoldRed, ColorReset, p.label, ColorGrayBold, size, ColorReset)
		return
	}
	fmt.Printf("  %s✔%s %s %s%s, %s%s\n", ColorBoldGreen, ColorReset, p.label, ColorGrayBold, size, elapsed(time.Since(p.start)), ColorReset)
}

// --- Step headers ---------------------------------------------------------------

// Step prints a section header such as "━━ 2/5  Checking your system ━━━".
func Step(n, total int, title string) {
	termMu.Lock()
	defer termMu.Unlock()
	clearLineLocked()
	head := fmt.Sprintf(" %d/%d  %s ", n, total, title)
	rule := 64 - utf8.RuneCountInString(head) - 2
	if rule < 3 {
		rule = 3
	}
	fmt.Printf("\n%s━━%s%s%s%s%s\n\n", ColorBoldBlue, ColorReset+ColorBold, head, ColorReset+ColorBoldBlue, strings.Repeat("━", rule), ColorReset)
}

// --- Logo banner ------------------------------------------------------------------

// logoCrop frames the "B" in the 1200x1200 launcher icon.
var logoCrop = image.Rect(300, 170, 900, 1030)

// logoPixels box-filters the crop into cols x rows pixels (rows even), dims
// the photo behind the letter and lifts the letter itself.
func logoPixels(img image.Image, cols int) [][]color.RGBA {
	b := img.Bounds()
	crop := image.Rect(
		b.Min.X+logoCrop.Min.X*b.Dx()/1200, b.Min.Y+logoCrop.Min.Y*b.Dy()/1200,
		b.Min.X+logoCrop.Max.X*b.Dx()/1200, b.Min.Y+logoCrop.Max.Y*b.Dy()/1200)
	rows := (cols*crop.Dy()/crop.Dx() + 1) / 2 * 2
	out := make([][]color.RGBA, rows)
	for y := 0; y < rows; y++ {
		out[y] = make([]color.RGBA, cols)
		for x := 0; x < cols; x++ {
			x0, x1 := crop.Min.X+x*crop.Dx()/cols, crop.Min.X+(x+1)*crop.Dx()/cols
			y0, y1 := crop.Min.Y+y*crop.Dy()/rows, crop.Min.Y+(y+1)*crop.Dy()/rows
			var r, g, bl, n float64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					R, G, B, _ := img.At(xx, yy).RGBA()
					r, g, bl, n = r+float64(R>>8), g+float64(G>>8), bl+float64(B>>8), n+1
				}
			}
			if n == 0 {
				n = 1
			}
			r, g, bl = r/n, g/n, bl/n
			k := 0.45
			if 0.2126*r+0.7152*g+0.0722*bl > 150 {
				k = 1.15
			}
			c := func(v float64) uint8 {
				if v *= k; v > 255 {
					v = 255
				}
				return uint8(v)
			}
			out[y][x] = color.RGBA{c(r), c(g), c(bl), 255}
		}
	}
	return out
}

func ansiColor(c color.RGBA, background bool) string {
	if trueColor() {
		if background {
			return fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B)
		}
		return fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B)
	}
	// Nearest colour in the xterm-256 6x6x6 cube or grey ramp.
	idx := func(v uint8) int { return (int(v)*5 + 127) / 255 }
	code := 16 + 36*idx(c.R) + 6*idx(c.G) + idx(c.B)
	if max(c.R, c.G, c.B)-min(c.R, c.G, c.B) < 12 {
		code = 232 + int(c.R)*23/255
	}
	if background {
		return fmt.Sprintf("\033[48;5;%dm", code)
	}
	return fmt.Sprintf("\033[38;5;%dm", code)
}

// LogoLines renders the launcher icon's "B" as half-block characters, two
// pixels per cell. It returns nil when the icon can't be read.
func LogoLines(iconPath string, cols int) []string {
	f, err := os.Open(iconPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil
	}
	px := logoPixels(img, cols)
	var lines []string
	for y := 0; y+1 < len(px); y += 2 {
		var b strings.Builder
		for x := range px[y] {
			b.WriteString(ansiColor(px[y][x], false) + ansiColor(px[y+1][x], true) + "▀")
		}
		b.WriteString(ColorReset)
		lines = append(lines, b.String())
	}
	return lines
}

// Banner shows the logo with info lines beside it, revealing the logo row by
// row. Without a fancy terminal, or without a logo, it prints the info only.
func Banner(logo []string, info []string) {
	termMu.Lock()
	defer termMu.Unlock()
	fmt.Println()
	if !Fancy() || len(logo) == 0 {
		for _, l := range info {
			fmt.Println("  " + l)
		}
		fmt.Println()
		return
	}
	offset := (len(logo) - len(info)) / 2
	if offset < 0 {
		offset = 0
	}
	for i, row := range logo {
		line := "  " + row
		if j := i - offset; j >= 0 && j < len(info) {
			line += "   " + info[j]
		}
		fmt.Println(line)
		time.Sleep(18 * time.Millisecond)
	}
	fmt.Println()
}
