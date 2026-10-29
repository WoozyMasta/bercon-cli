// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package config

import (
	"testing"
	"time"
)

// TestDurationUnmarshalText supports seconds as the legacy unit and Go duration suffixes.
func TestDurationUnmarshalText(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{value: "3", want: 3 * time.Second},
		{value: "250ms", want: 250 * time.Millisecond},
		{value: "2m", want: 2 * time.Minute},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			var got Duration
			if err := got.UnmarshalText([]byte(test.value)); err != nil {
				t.Fatal(err)
			}
			if time.Duration(got) != test.want {
				t.Fatalf("duration = %s, want %s", got, test.want)
			}
		})
	}
}

// TestMillisecondDurationUnmarshalText preserves legacy command-delay units.
func TestMillisecondDurationUnmarshalText(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{value: "1", want: time.Millisecond},
		{value: "250ms", want: 250 * time.Millisecond},
		{value: "2s", want: 2 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			var got MillisecondDuration
			if err := got.UnmarshalText([]byte(test.value)); err != nil {
				t.Fatal(err)
			}
			if time.Duration(got) != test.want {
				t.Fatalf("duration = %s, want %s", got, test.want)
			}
		})
	}
}

// TestDurationUnmarshalTextRejectsInvalidValues prevents invalid text and overflow becoming zero.
func TestDurationUnmarshalTextRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "fast", "9223372036854775807"} {
		t.Run(value, func(t *testing.T) {
			var got Duration
			if err := got.UnmarshalText([]byte(value)); err == nil {
				t.Fatalf("duration %q was accepted", value)
			}
		})
	}
}
