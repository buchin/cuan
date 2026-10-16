package sponsorship

import (
	"testing"
)

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text without tags",
			input:    "Dropsugar Studio",
			expected: "Dropsugar Studio",
		},
		{
			name:     "strips script tags",
			input:    "Acme <script>alert('pwned')</script> Corp",
			expected: "Acme alert('pwned') Corp",
		},
		{
			name:     "strips img onerror handler tags",
			input:    `Awesome Slogan <img src="x" onerror="alert(1)"> for solo devs`,
			expected: "Awesome Slogan for solo devs",
		},
		{
			name:     "strips nested tags",
			input:    `<b><i>Empowering</i></b> Founders <iframe src="evil.com"></iframe>`,
			expected: "Empowering Founders",
		},
		{
			name:     "removes control characters",
			input:    "Brand\x00\x08Name\x1F",
			expected: "BrandName",
		},
		{
			name:     "normalizes excess whitespace",
			input:    "   Leading   and    trailing   spaces   ",
			expected: "Leading and trailing spaces",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeText(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestValidateAndFormatURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
		expectErr   bool
	}{
		{
			name:        "valid https url",
			input:       "https://cuan.co",
			expectedURL: "https://cuan.co",
			expectErr:   false,
		},
		{
			name:        "auto prepends https to domain without protocol",
			input:       "dropsugar.com/cuan",
			expectedURL: "https://dropsugar.com/cuan",
			expectErr:   false,
		},
		{
			name:        "valid http url",
			input:       "http://myindieproject.dev",
			expectedURL: "http://myindieproject.dev",
			expectErr:   false,
		},
		{
			name:      "rejects javascript scheme",
			input:     "javascript:alert(1)",
			expectErr: true,
		},
		{
			name:      "rejects data scheme",
			input:     "data:text/html,<script>alert(1)</script>",
			expectErr: true,
		},
		{
			name:      "rejects empty url",
			input:     "   ",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ValidateAndFormatURL(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (%s)", tc.input, res)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tc.input, err)
				}
				if res != tc.expectedURL {
					t.Errorf("expected %q, got %q", tc.expectedURL, res)
				}
			}
		})
	}
}

func TestValidateContactEmail(t *testing.T) {
	valid := []string{"buchin@dropsugar.com", "dev.team+s2@cuan.co", "founder@sub.domain.id"}
	for _, em := range valid {
		if err := ValidateContactEmail(em); err != nil {
			t.Errorf("expected email %q to be valid, got: %v", em, err)
		}
	}

	invalid := []string{"not-an-email", "missing-domain@", "@missing-user.com", "plainaddress", "   "}
	for _, em := range invalid {
		if err := ValidateContactEmail(em); err == nil {
			t.Errorf("expected email %q to fail validation", em)
		}
	}
}
