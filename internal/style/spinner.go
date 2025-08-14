package style

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type Spinner interface {
	SetSuffix(suffix string)
	SetFinalMSG(finalMSG string)
	Start()
	Stop()
}

// TestSpinner is a spinner implementation for testing that outputs each
// spinner update on a new line instead of clearing and redrawing
type TestSpinner struct {
	ID         string
	mu         *sync.RWMutex
	Delay      time.Duration
	chars      []string
	Prefix     string
	Suffix     string
	FinalMSG   string
	color      func(a ...interface{}) string
	Writer     io.Writer
	active     bool
	enabled    bool
	stopChan   chan struct{}
	HideCursor bool // kept for interface compatibility but ignored
	PreUpdate  func(s *TestSpinner)
	PostUpdate func(s *TestSpinner)
}

type TestOption func(*TestSpinner)

// New provides a pointer to an instance of TestSpinner with the supplied options.
func NewTestSpinner(cs []string, d time.Duration, options ...TestOption) *TestSpinner {
	s := &TestSpinner{
		Delay:      d,
		chars:      cs,
		color:      color.New(color.FgWhite).SprintFunc(),
		mu:         &sync.RWMutex{},
		Writer:     os.Stdout,
		stopChan:   make(chan struct{}, 1),
		active:     false,
		enabled:    true,
		HideCursor: true,
	}

	for _, option := range options {
		option(s)
	}

	return s
}

func (s *TestSpinner) SetSuffix(suffix string) {
	if !s.active {
		s.Suffix = suffix
		return
	}

	oldSuffix := s.Suffix

	// No change, don't print anything
	if oldSuffix == suffix {
		return
	}

	// Create diff using diffmatchpatch
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldSuffix, suffix, false)

	// Cleanup the diffs for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Format the diff output
	var diffParts []string
	hasChanges := false

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffDelete:
			if diff.Text != "" {
				diffParts = append(diffParts, fmt.Sprintf("-%s", diff.Text))
				hasChanges = true
			}
		case diffmatchpatch.DiffInsert:
			if diff.Text != "" {
				diffParts = append(diffParts, fmt.Sprintf("+%s", diff.Text))
				hasChanges = true
			}
		}
	}

	// Only print if there are actual changes
	if hasChanges {
		diffOutput := strings.Join(diffParts, "")
		fmt.Fprintf(s.Writer, "[%s] %s\n", s.ID, strings.TrimRight(diffOutput, "\n"))
	}

	s.Suffix = suffix
}

// Start will start the indicator.
func (s *TestSpinner) Start() {
	s.mu.Lock()
	if s.active || !s.enabled {
		s.mu.Unlock()
		return
	}

	s.active = true
	s.mu.Unlock()

	// Output start message
	fmt.Fprintf(s.Writer, "[Spinner %s]:%s\n", s.ID, s.Suffix)
}

// Stop stops the indicator.
func (s *TestSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.active = false

		fmt.Fprintf(s.Writer, "[Finish %s]%s\n", s.ID, s.FinalMSG)

		s.stopChan <- struct{}{}
	}
}

func (s *TestSpinner) SetFinalMSG(finalMSG string) {
	s.FinalMSG = finalMSG
}

// Color will set the struct field for the given color to be used.
func (s *TestSpinner) Color(colors ...string) error {
	colorAttributes := make([]color.Attribute, len(colors))

	// Basic color validation
	validColors := map[string]color.Attribute{
		"black":   color.FgBlack,
		"red":     color.FgRed,
		"green":   color.FgGreen,
		"yellow":  color.FgYellow,
		"blue":    color.FgBlue,
		"magenta": color.FgMagenta,
		"cyan":    color.FgCyan,
		"white":   color.FgWhite,
	}

	for index, c := range colors {
		if attr, ok := validColors[c]; ok {
			colorAttributes[index] = attr
		} else {
			return errors.New("invalid color")
		}
	}

	s.mu.Lock()
	s.color = color.New(colorAttributes...).SprintFunc()
	s.mu.Unlock()

	fmt.Fprintf(s.Writer, "[COLOR CHANGED] %s\n", strings.Join(colors, ", "))
	return nil
}

// UpdateSpeed will set the indicator delay to the given value.
func (s *TestSpinner) UpdateSpeed(d time.Duration) {
	s.mu.Lock()
	oldDelay := s.Delay
	s.Delay = d
	s.mu.Unlock()

	fmt.Fprintf(s.Writer, "[SPEED UPDATED] %v -> %v\n", oldDelay, d)
}

// UpdateCharSet will change the current character set to the given one.
func (s *TestSpinner) UpdateCharSet(cs []string) {
	s.mu.Lock()
	s.chars = cs
	s.mu.Unlock()

	fmt.Fprintf(s.Writer, "[CHARSET UPDATED] %v\n", cs)
}

// Lock allows for manual control to lock the spinner.
func (s *TestSpinner) Lock() {
	s.mu.Lock()
}

// Unlock allows for manual control to unlock the spinner.
func (s *TestSpinner) Unlock() {
	s.mu.Unlock()
}

type TerminalSpinner struct {
	spinner *spinner.Spinner
}

func NewTerminalSpinner(cs []string, d time.Duration, options ...spinner.Option) *TerminalSpinner {
	return &TerminalSpinner{
		spinner: spinner.New(cs, d, options...),
	}
}

func (s *TerminalSpinner) SetSuffix(suffix string) {
	s.spinner.Suffix = suffix
}

func (s *TerminalSpinner) SetFinalMSG(finalMSG string) {
	s.spinner.FinalMSG = finalMSG
}

func (s *TerminalSpinner) Start() {
	s.spinner.Start()
}

func (s *TerminalSpinner) Stop() {
	s.spinner.Stop()
}

type SpinnerManager struct {
	mu      *sync.Mutex
	writer  io.Writer
	counter int
}

func NewSpinnerManager(w io.Writer) *SpinnerManager {
	return &SpinnerManager{
		writer: w,
		mu:     &sync.Mutex{},
	}
}

func (s *SpinnerManager) Start() Spinner {
	s.mu.Lock()
	defer func() {
		s.counter++
		s.mu.Unlock()
	}()

	if os.Getenv("LACQUER_TEST") == "true" {
		return NewTestSpinner(spinner.CharSets[9], 100*time.Millisecond, func(ts *TestSpinner) {
			ts.Writer = s.writer
			ts.ID = fmt.Sprintf("spinner-%d", s.counter)
		})
	}

	return NewTerminalSpinner(spinner.CharSets[9], 100*time.Millisecond, spinner.WithWriter(s.writer))
}
