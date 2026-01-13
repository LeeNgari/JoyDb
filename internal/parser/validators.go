package parser

import (
	"fmt"
	"strings"
	"time"
)

// validateDate validates a date string in YYYY-MM-DD format
func validateDate(value string) error {
	_, err := time.Parse("2006-01-02", value)
	if err != nil {
		return fmt.Errorf("invalid date format, expected YYYY-MM-DD (e.g., '2024-01-13')")
	}
	return nil
}

// validateTime validates a time string in HH:MM:SS or HH:MM format
func validateTime(value string) error {
	// Try HH:MM:SS format first
	_, err := time.Parse("15:04:05", value)
	if err != nil {
		// Try HH:MM format
		_, err = time.Parse("15:04", value)
		if err != nil {
			return fmt.Errorf("invalid time format, expected HH:MM:SS or HH:MM (e.g., '14:30:00' or '14:30')")
		}
	}
	return nil
}

// validateEmail validates an email string with basic format checking
func validateEmail(value string) error {
	// Basic validation: must have @ and domain with dot
	if !strings.Contains(value, "@") {
		return fmt.Errorf("email must contain @ symbol")
	}

	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return fmt.Errorf("email must have exactly one @ symbol")
	}

	localPart := parts[0]
	domain := parts[1]

	if localPart == "" {
		return fmt.Errorf("email local part (before @) cannot be empty")
	}

	if domain == "" {
		return fmt.Errorf("email domain (after @) cannot be empty")
	}

	if !strings.Contains(domain, ".") {
		return fmt.Errorf("email domain must contain a dot (e.g., 'example.com')")
	}

	// Check domain doesn't start or end with dot
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("email domain cannot start or end with a dot")
	}

	return nil
}
