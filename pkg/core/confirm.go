package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Scanner is a wrapper around bufio.Scanner for reading user input
type Scanner struct {
	reader *bufio.Reader
}

// NewReader creates a new Scanner for reading from stdin
func NewReader() *Scanner {
	return &Scanner{reader: bufio.NewReader(os.Stdin)}
}

// ReadString reads a string terminated by a delimiter
func (s *Scanner) ReadString(delim byte) (string, error) {
	return s.reader.ReadString(delim)
}

// AssumeYes answers every default-yes question with its default, for the
// installer's --yes flag. Destructive default-no questions are unaffected.
var AssumeYes bool

// Prompt asks for a line of text and returns def when the user just presses
// Enter, when input is unavailable, or when AssumeYes is set.
func Prompt(prompt, def string) string {
	fmt.Print(prompt)
	if AssumeYes {
		fmt.Println(def)
		return def
	}
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	input = strings.TrimSpace(input)
	if err != nil && input == "" {
		fmt.Println()
		return def
	}
	if input == "" {
		return def
	}
	return input
}

// AskBool prompts the user for confirmation and returns true if they confirm
// If the user presses Enter without typing anything, it defaults to true (yes)
func AskBool(prompt string) bool {
	fmt.Printf("  %s?%s %s", ColorBoldCyan, ColorReset, prompt)
	if AssumeYes {
		fmt.Println("y")
		return true
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return false
	}

	input = strings.TrimSpace(input)

	if input == "" {
		// Default to yes (empty input)
		return true
	}

	inputLower := strings.ToLower(input)
	if inputLower == "y" || inputLower == "yes" {
		return true
	}
	if inputLower == "n" || inputLower == "no" {
		return false
	}

	fmt.Printf("%sInvalid input: '%s'%s\n", ColorBoldRed, input, ColorReset)
	fmt.Println("Please enter 'y' or 'Y' to proceed, 'n' or 'N' to cancel, or press Enter to proceed (default: yes)")
	return AskBool(prompt)
}

// AskBoolDefaultNo requires an explicit yes for destructive actions.
func AskBoolDefaultNo(prompt string) bool {
	fmt.Print(prompt)
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
