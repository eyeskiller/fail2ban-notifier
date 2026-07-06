package setup

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

func parseIndex(s string) int {
	idx, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return idx - 1
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

func maskString(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

func (w *SetupWizard) promptInput(prompt string, defaultValue string, secret bool) string {
	for {
		fmt.Print(prompt)
		var val string
		if secret {
			byteVal, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
				return defaultValue
			}
			val = string(byteVal)
		} else {
			input, _ := w.reader.ReadString('\n')
			val = strings.TrimSpace(input)
		}

		if val == "" && defaultValue != "" {
			return defaultValue
		}
		return val
	}
}

func (w *SetupWizard) promptConfirm(prompt string, defaultYes bool) bool {
	yn := "Y/n"
	if !defaultYes {
		yn = "y/N"
	}
	fmt.Printf("%s [%s] ", prompt, yn)
	input, _ := w.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}
