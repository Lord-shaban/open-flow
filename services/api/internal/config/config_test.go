package config

import "testing"

func TestDefaults(t *testing.T) {
	for _, key := range []string{"OPEN_FLOW_HTTP_ADDR", "OPEN_FLOW_LOG_LEVEL", "OPEN_FLOW_TEMPORAL_ADDRESS", "OPEN_FLOW_KAFKA_BROKER"} {
		t.Setenv(key, "")
	}
	c, err := Load()
	if err != nil || c.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected defaults: %+v, %v", c, err)
	}
}

func TestRejectInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"OPEN_FLOW_HTTP_ADDR", "invalid"},
		{"OPEN_FLOW_TEMPORAL_ADDRESS", "bad"},
		{"OPEN_FLOW_KAFKA_BROKER", "bad"},
		{"OPEN_FLOW_LOG_LEVEL", "secrets"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid configuration to fail")
			}
		})
	}
}
