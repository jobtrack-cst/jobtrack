package main

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestConfig_RejectsUnsafeInternalSettings(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"SERVICE_NAME", "offer"},
		{"INTERNAL_TOKEN", ""}, {"INTERNAL_TOKEN", "secret\nvalue"}, {"INTERNAL_TOKEN", "secret value"},
		{"CATEGORY_TAG_ADDR", ""},
		{"CATEGORY_TAG_ADDR", "bad\x00host:9090"},
		{"CATEGORY_TAG_ADDR", "bad_host:9090"}, {"CATEGORY_TAG_ADDR", "category-tag"}, {"CATEGORY_TAG_ADDR", "host:0"}, {"CATEGORY_TAG_ADDR", "host:65536"}, {"CATEGORY_TAG_ADDR", "http://host:9090"},
		{"RABBITMQ_URL", "https://broker"}, {"RABBITMQ_URL", "amqp://"},
	} {
		t.Run(tc.key+" "+strings.ReplaceAll(tc.value, "\n", " "), func(t *testing.T) {
			env := map[string]string{"SERVICE_NAME": "job-alert", "DATABASE_URL": "postgres://localhost/test", "RABBITMQ_URL": "amqp://localhost", "INTERNAL_TOKEN": "synthetic-token", "CATEGORY_TAG_ADDR": "category-tag:9090"}
			env[tc.key] = tc.value
			_, err := loadConfig(func(key string) string { return env[key] })
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.key)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}
