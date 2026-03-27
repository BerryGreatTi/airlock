package secrets

import "testing"

func TestIsSecret(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		expect bool
	}{
		// Key name matches
		{name: "key contains TOKEN", key: "SLACK_TOKEN", value: "xoxb-1234567890-abcdef", expect: true},
		{name: "key contains KEY", key: "API_KEY", value: "some-long-api-value", expect: true},
		{name: "key contains SECRET", key: "WEBHOOK_SECRET", value: "whsec_abcdefghijk", expect: true},
		{name: "key contains PASSWORD", key: "DB_PASSWORD", value: "p4ssw0rd-long-enough", expect: true},
		{name: "key contains CREDENTIAL", key: "MY_CREDENTIAL", value: "cred-abcdefghijk", expect: true},
		{name: "key contains AUTH", key: "AUTH_BEARER", value: "bearer-token-value", expect: true},
		{name: "key case insensitive", key: "slack_token", value: "xoxb-1234-abcdef", expect: true},
		// Value prefix matches
		{name: "sk- prefix", key: "STRIPE", value: "sk-ant-api03-abcdefgh", expect: true},
		{name: "sk_live_ prefix", key: "STRIPE", value: "sk_live_abcdefghijk", expect: true},
		{name: "sk_test_ prefix", key: "STRIPE", value: "sk_test_abcdefghijk", expect: true},
		{name: "pk_live_ prefix", key: "STRIPE", value: "pk_live_abcdefghijk", expect: true},
		{name: "xoxb- prefix", key: "SLACK", value: "xoxb-123-456-abcdef", expect: true},
		{name: "xoxp- prefix", key: "SLACK", value: "xoxp-123-456-abcdef", expect: true},
		{name: "ghp_ prefix", key: "GH", value: "ghp_abcdefghijklmnop", expect: true},
		{name: "gho_ prefix", key: "GH", value: "gho_abcdefghijklmnop", expect: true},
		{name: "ghs_ prefix", key: "GH", value: "ghs_abcdefghijklmnop", expect: true},
		{name: "ghu_ prefix", key: "GH", value: "ghu_abcdefghijklmnop", expect: true},
		{name: "glpat- prefix", key: "GL", value: "glpat-abcdefghijklmn", expect: true},
		{name: "AKIA prefix", key: "AWS", value: "AKIAIOSFODNN7EXAMPLE", expect: true},
		{name: "eyJ prefix", key: "JWT", value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", expect: true},
		{name: "whsec_ prefix", key: "WH", value: "whsec_abcdefghijklm", expect: true},
		{name: "rk_live_ prefix", key: "STRIPE", value: "rk_live_abcdefghijk", expect: true},
		{name: "rk_test_ prefix", key: "STRIPE", value: "rk_test_abcdefghijk", expect: true},
		// Exclusions
		{name: "short value", key: "API_KEY", value: "short", expect: false},
		{name: "boolean true", key: "AUTH_ENABLED", value: "true", expect: false},
		{name: "boolean false", key: "AUTH_ENABLED", value: "false", expect: false},
		{name: "number 0", key: "TOKEN_COUNT", value: "0", expect: false},
		{name: "number 1", key: "TOKEN_COUNT", value: "1", expect: false},
		{name: "path value", key: "SECRET_PATH", value: "/usr/local/bin/node", expect: false},
		{name: "url value", key: "AUTH_URL", value: "https://auth.example.com", expect: false},
		{name: "http url", key: "AUTH_URL", value: "http://localhost:8080", expect: false},
		// Not a secret
		{name: "generic key and value", key: "LOG_LEVEL", value: "production", expect: false},
		{name: "region value", key: "AWS_REGION", value: "us-east-1", expect: false},
		{name: "empty value", key: "API_KEY", value: "", expect: false},
		{name: "feature flag", key: "ENABLE_FEATURE", value: "enabled", expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSecret(tt.key, tt.value)
			if got != tt.expect {
				t.Errorf("IsSecret(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.expect)
			}
		})
	}
}
