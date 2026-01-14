package errors

import "fmt"

// ParseError represents an error during SQL parsing
type ParseError struct {
	Message  string
	Token    string // The problematic token
	Line     int    // Line number (if available)
	Column   int    // Column number (if available)
	Cause    error  // Underlying error (if any)
}

func (e *ParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("parse error at line %d, column %d: %s (token: %s)", 
			e.Line, e.Column, e.Message, e.Token)
	}
	if e.Token != "" {
		return fmt.Sprintf("parse error: %s (token: %s)", e.Message, e.Token)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NewParseError creates a new parse error
func NewParseError(message, token string) *ParseError {
	return &ParseError{
		Message: message,
		Token:   token,
	}
}

// NewParseErrorWithPosition creates a parse error with position information
func NewParseErrorWithPosition(message, token string, line, column int) *ParseError {
	return &ParseError{
		Message: message,
		Token:   token,
		Line:    line,
		Column:  column,
	}
}

// NewParseErrorWithCause creates a parse error wrapping another error
func NewParseErrorWithCause(message string, cause error) *ParseError {
	return &ParseError{
		Message: message,
		Cause:   cause,
	}
}
