// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/woozymasta/flags"
)

// localeFiles contains the application translation catalogs.
//
//go:embed locales/*.json
var localeFiles embed.FS

// applicationMessages is loaded once and shared by parser and output localizers.
var applicationMessages = mustLoadApplicationCatalog()

// catalog stores application-owned translations by locale and message key.
type catalog map[string]map[string]string

// Lookup returns the translation for a base language and message key.
func (c catalog) Lookup(locale, key string) (string, bool) {
	language, _, _ := strings.Cut(strings.ToLower(strings.ReplaceAll(locale, "_", "-")), "-")
	text, ok := c[language][key]

	return text, ok
}

// mustLoadApplicationCatalog reads embedded JSON catalogs and fails on invalid built-in data.
func mustLoadApplicationCatalog() catalog {
	catalogs := make(catalog)
	files, err := fs.Glob(localeFiles, "locales/*.json")
	if err != nil {
		panic(fmt.Errorf("list locale catalogs: %w", err))
	}

	for _, name := range files {
		data, err := localeFiles.ReadFile(name)
		if err != nil {
			panic(fmt.Errorf("read locale catalog %q: %w", name, err))
		}

		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			panic(fmt.Errorf("decode locale catalog %q: %w", name, err))
		}

		catalogs[strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))] = messages
	}

	return catalogs
}

// i18nConfig connects application JSON messages to the flags package localizer.
func i18nConfig() flags.I18nConfig {
	return flags.I18nConfig{
		FallbackLocales: []string{"en"},
		UserCatalog:     applicationMessages,
	}
}

// appText localizes one application-owned message using the detected system locale.
func appText(key, fallback string) string {
	return flags.NewLocalizer(i18nConfig()).Localize(key, fallback, nil)
}

// appError formats a localized application error with optional values.
func appError(key, fallback string, args ...any) error {
	return fmt.Errorf(appText(key, fallback), args...)
}

// appWrap adds localized context while preserving the underlying error for errors.Is.
func appWrap(key, fallback string, err error) error {
	return fmt.Errorf("%s: %w", appText(key, fallback), err)
}
