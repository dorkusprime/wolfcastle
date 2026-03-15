package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

	// paused is set by clearForMessage to suppress redraws until
	// the next message has been written and a grace period passes.
	paused atomic.Bool
}

const (
	spinnerWidth = 20       // total characters inside the brackets
	projectile   = ">>──▶"  // the moving round
	frameDelay   = 80 * time.Millisecond
)

// Global coordination: PrintHuman and PauseSpinner check this before writing.
var (
	activeMu      sync.Mutex
	activeSpinner *Spinner
)

// PauseSpinner temporarily clears the spinner so other output can
// print cleanly. Call ResumeSpinner after writing. Safe to call
// when no spinner is active (no-op).
func PauseSpinner() {
	activeMu.Lock()
	s := activeSpinner
	activeMu.Unlock()
	if s != nil {
		s.clearForMessage()
	}
}

// ResumeSpinner re-enables the spinner after a PauseSpinner call.
// The spinner stays suppressed briefly so rapid-fire messages don't
// interleave with redraws.
func ResumeSpinner() {
	activeMu.Lock()
	s := activeSpinner
	activeMu.Unlock()
	if s != nil {
		s.resumeAfterMessage()
	}
}

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

// clearForMessage is called by PrintHuman to erase the spinner line
// and suppress redraws until the message is written.
func (s *Spinner) clearForMessage() {
	s.paused.Store(true)
	fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", spinnerWidth+1))
}

// resumeAfterMessage is called by PrintHuman after writing the message.
// The spinner stays paused for a short grace period so rapid-fire
// messages don't interleave with redraws.
func (s *Spinner) resumeAfterMessage() {
	go func() {
		time.Sleep(frameDelay * 2)
		s.paused.Store(false)
	}()
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
			if s.paused.Load() {
				continue
			}
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
