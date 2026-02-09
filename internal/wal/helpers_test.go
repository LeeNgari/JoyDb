package wal

import (
	"os"
	"testing"
)

func TestAlignTo8_Examples(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 0},
		{1, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{33, 40},
	}

	for _, tc := range tests {
		got := AlignTo8(tc.input)
		if got != tc.expected {
			t.Errorf("AlignTo8(%d) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func TestCalculateFileCRC32(t *testing.T) {
	// Create temp file
	f, _ := os.CreateTemp("", "crc_test")
	defer os.Remove(f.Name())
	f.Write([]byte("hello"))
	f.Close()

	crc, err := CalculateFileCRC32(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	// CRC32 of "hello"
	if crc == 0 {
		t.Error("CRC should not be 0")
	}
}
