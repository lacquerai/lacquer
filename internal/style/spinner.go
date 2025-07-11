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
	fmt.Fprintf(s.Writer, "[SET SUFFIX] %s\n", suffix)
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
	fmt.Fprintf(s.Writer, "[SPINNER START]\n")
}

// Stop stops the indicator.
func (s *TestSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.active = false

		// Output stop message
		fmt.Fprintf(s.Writer, "[SPINNER STOP]\n")

		if s.FinalMSG != "" {
			fmt.Fprintf(s.Writer, "[FINAL MSG] %s\n", s.FinalMSG)
		}

		s.stopChan <- struct{}{}
	}
}

// Restart will stop and start the indicator.
func (s *TestSpinner) Restart() {
	s.Stop()
	time.Sleep(10 * time.Millisecond) // small delay to ensure clean restart
	s.Start()
}

// Reverse will reverse the order of the slice assigned to the indicator.
func (s *TestSpinner) Reverse() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, j := 0, len(s.chars)-1; i < j; i, j = i+1, j-1 {
		s.chars[i], s.chars[j] = s.chars[j], s.chars[i]
	}
	fmt.Fprintf(s.Writer, "[REVERSED CHARSET]\n")
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

func NewSpinner(w io.Writer) Spinner {
	if os.Getenv("LACQUER_TEST") == "true" {
		return NewTestSpinner(spinner.CharSets[9], 100*time.Millisecond, func(s *TestSpinner) {
			s.Writer = w
		})
	}

	return NewTerminalSpinner(spinner.CharSets[9], 100*time.Millisecond, spinner.WithWriter(w))
}
