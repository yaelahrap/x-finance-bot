// Package config provides configuration loading and access for the bot.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// getEnv returns the value of the given environment variable, or fallback if unset or empty.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getEnvInt returns the integer value of the given environment variable, or fallback
// if unset, empty, or unparseable.
func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// getEnvBool returns the boolean value of the given environment variable, or fallback
// if unset, empty, or unparseable. Accepts standard Go truthy values (1, t, T, TRUE, true, True).
func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// getEnvDuration returns the time.Duration value of the given environment variable, or
// fallback if unset, empty, or unparseable. The value must be a Go duration string
// (e.g. "5s", "10m", "1h30m").
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// mustGetEnv returns the value of the given environment variable. It returns an error
// if the variable is unset or empty so callers can aggregate missing-variable diagnostics.
func mustGetEnv(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}
