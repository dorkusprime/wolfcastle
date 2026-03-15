package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Spinner renders a repeating projectile animation on a single line.
// The round travels left-to-right across a fixed-width track, then
// wraps back to the start. Call Stop() to clear the line and halt.
//
// While a spinner is active, PrintHuman clears the animation before
// writing so messages never collide with the spinner frame.
type Spinner struct {
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	started bool
}

const (
	spinnerWidth = 20       // total characters inside the brackets
	projectile   = ">>──▶"  // the moving round
	frameDelay   = 80 * time.Millisecond
)

// Global coordination: PrintHuman checks this before writing.
var (
	activeMu      sync.Mutex
	activeSpinner *Spinner
)

// NewSpinner creates a spinner but does not start it.
func NewSpinner() *Spinner {
	return &Spinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Start begins the animation in a background goroutine.
// Does nothing if stdout is not a terminal. Safe to call multiple times.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}
	s.started = true

	if !IsTerminal() {
		close(s.done)
		return
	}

	activeMu.Lock()
	activeSpinner = s
	activeMu.Unlock()

	go s.run()
}

// Stop halts the animation and clears the spinner line.
// Safe to call without a preceding Start().
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		close(s.done)
		return
	}
	s.mu.Unlock()

	select {
	case s.stop <- struct{}{}:
	default:
	}
	<-s.done

	activeMu.Lock()
	if activeSpinner == s {
		activeSpinner = nil
	}
	activeMu.Unlock()
}

// clearForMessage is called by PrintHuman to temporarily erase the
// spinner line so the message prints cleanly. The spinner's next
// tick redraws itself automatically.
func (s *Spinner) clearForMessage() {
	fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", spinnerWidth+1))
}

func (s *Spinner) run() {
	defer close(s.done)

	// Hide cursor while animating to prevent flicker.
	fmt.Fprint(os.Stdout, "\033[?25l")
	defer fmt.Fprint(os.Stdout, "\033[?25h")

	projLen := len([]rune(projectile))
	pos := 0
	ticker := time.NewTicker(frameDelay)
	defer ticker.Stop()

	// Render first frame immediately.
	fmt.Fprintf(os.Stdout, "\r%s", renderFrame(pos, projLen))
	pos = (pos + 1) % spinnerWidth

	for {
		select {
		case <-s.stop:
			// Erase the spinner line.
			fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", spinnerWidth+1))
			return
		case <-ticker.C:
			fmt.Fprintf(os.Stdout, "\r%s", renderFrame(pos, projLen))
			pos = (pos + 1) % spinnerWidth
		}
	}
}

// renderFrame builds one animation frame.
// The projectile wraps smoothly when it reaches the right edge.
func renderFrame(pos, projLen int) string {
	track := make([]rune, spinnerWidth)
	for i := range track {
		track[i] = ' '
	}

	proj := []rune(projectile)
	for i, ch := range proj {
		idx := (pos + i) % spinnerWidth
		track[idx] = ch
	}

	return "" + string(track) + "|"
}

// IsTerminal reports whether stdout is connected to a terminal.
// Uses os.File.Stat() to check for ModeCharDevice, which works
// across platforms without platform-specific ioctl constants.
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
