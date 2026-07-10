package joydb

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrClosed           = errors.New("joydb: store is closed")
	ErrTxDone           = errors.New("joydb: transaction is already closed")
	ErrInvalidName      = errors.New("joydb: invalid database name")
	ErrTableNotFound    = errors.New("joydb: table not found")
	ErrUniqueConstraint = errors.New("joydb: unique constraint violation")
	ErrNotNull          = errors.New("joydb: not null constraint violation")
	ErrPrimaryKey       = errors.New("joydb: primary key constraint violation")
	ErrTypeConversion   = errors.New("joydb: type conversion failed")
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "table not found"):
		return fmt.Errorf("%w: %v", ErrTableNotFound, err)
	case strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate"):
		return fmt.Errorf("%w: %v", ErrUniqueConstraint, err)
	case strings.Contains(lower, "not null"):
		return fmt.Errorf("%w: %v", ErrNotNull, err)
	case strings.Contains(lower, "primary key"):
		return fmt.Errorf("%w: %v", ErrPrimaryKey, err)
	case strings.Contains(lower, "convert") || strings.Contains(lower, "type mismatch"):
		return fmt.Errorf("%w: %v", ErrTypeConversion, err)
	default:
		return err
	}
}
