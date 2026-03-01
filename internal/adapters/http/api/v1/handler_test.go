package v1

import "testing"

func TestMaskEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "common", email: "admin@example.com", want: "a****@***e.com"},
		{name: "spaces", email: "  user@test.dev ", want: "u****@***t.dev"},
		{name: "invalid", email: "invalid", want: "invalid"},
		{name: "empty-local", email: "@example.com", want: "@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskEmail(tc.email); got != tc.want {
				t.Fatalf("maskEmail(%q) = %q, want %q", tc.email, got, tc.want)
			}
		})
	}
}
