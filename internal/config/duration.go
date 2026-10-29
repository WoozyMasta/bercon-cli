// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package config

import (
	"strconv"
	"time"
)

// maxInt64 bounds unitless values before applying their default time unit.
const maxInt64 int64 = 1<<63 - 1

// minInt64 bounds unitless values before applying their default time unit.
const minInt64 int64 = -1 << 63

// Duration parses Go duration strings and treats unitless integers as seconds.
type Duration time.Duration

// MillisecondDuration parses Go duration strings and treats unitless integers as milliseconds.
type MillisecondDuration time.Duration

// UnmarshalText parses a duration, preserving seconds as the legacy unit for bare integers.
func (d *Duration) UnmarshalText(text []byte) error {
	value, err := parseDuration(text, time.Second)
	if err != nil {
		return err
	}

	*d = Duration(value)
	return nil
}

// MarshalText formats the duration using Go's standard duration syntax.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// String returns the duration using Go's standard duration syntax.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// UnmarshalText parses a duration, preserving milliseconds as the legacy unit for bare integers.
func (d *MillisecondDuration) UnmarshalText(text []byte) error {
	value, err := parseDuration(text, time.Millisecond)
	if err != nil {
		return err
	}

	*d = MillisecondDuration(value)
	return nil
}

// MarshalText formats the duration using Go's standard duration syntax.
func (d MillisecondDuration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// String returns the duration using Go's standard duration syntax.
func (d MillisecondDuration) String() string {
	return time.Duration(d).String()
}

// parseDuration applies the field's legacy unit only when the value has no suffix.
func parseDuration(text []byte, unit time.Duration) (time.Duration, error) {
	value := string(text)
	integer, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.ParseDuration(value)
	}

	unitValue := int64(unit)
	if integer > maxInt64/unitValue || integer < minInt64/unitValue {
		return 0, strconv.ErrRange
	}

	return time.Duration(integer * unitValue), nil
}
