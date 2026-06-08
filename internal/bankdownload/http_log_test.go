package bankdownload

import "testing"

func TestRedactHeaderValue(t *testing.T) {
	cases := []struct {
		key, value, want string
	}{
		{"Authorization", "Bearer secret", "(13 chars, redacted)"},
		{"Cookie", "a=b; c=d", "(8 chars, redacted)"},
		{"Set-Cookie", "session=abc; Path=/", "session=(redacted)"},
		{"Client-Id", "my-client", "my-client"},
		{"Cookie", "", "(empty)"},
	}
	for _, tc := range cases {
		got := redactHeaderValue(tc.key, tc.value)
		if got != tc.want {
			t.Errorf("redactHeaderValue(%q, %q) = %q, want %q", tc.key, tc.value, got, tc.want)
		}
	}
}
