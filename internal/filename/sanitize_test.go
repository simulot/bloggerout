package filename

import (
	"strings"
	"testing"
)

// TestSanitize runs a table-driven test to check the SanitizeFilename function.
func TestSanitize(t *testing.T) {
	// Define the test cases
	testCases := []struct {
		name     string // Name of the test case
		input    string // Input to the function
		expected string // Expected output
	}{
		{"illegal_chars", `a*b<c>d:e"f/g\h|i?j.txt`, "a_b_c_d_e_f_g_h_i_j.txt"},
		{"leading_trailing_spaces", "  filename with spaces  ", "filename with spaces"},
		{"leading_trailing_dots", "...filename.with.dots...", "filename.with.dots"},
		{"reserved_name_uppercase", "CON.txt", "_CON.txt"},
		{"reserved_name_lowercase", "prn.aux", "_prn.aux"},
		{"reserved_name_no_extension", "nul", "_nul"},
		{"multiple_underscores", "a__b___c.txt", "a_b_c.txt"},
		{"control_chars", string([]byte{0x01, 0x02}) + "text" + string([]byte{0x1f}), "text"},
		{"empty_input", "", "unnamed_file"},
		{"only_illegal_chars", "<>:/\\|?*\"", "unnamed_file"},
		{"only_dots_and_spaces", " . . . ", "unnamed_file"},
		{"long_filename", strings.Repeat("a", 300) + ".txt", strings.Repeat("a", 251) + ".txt"},
		{"long_filename_no_ext", strings.Repeat("b", 300), strings.Repeat("b", 255)},
		{"mixed_issues", "  /a\\b: <c>?.txt ", "_a_b_c_.txt"},
		{"clean_filename", "my-safe-filename.zip", "my-safe-filename.zip"},
	}

	// Loop through the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sanitize(tc.input)
			if got != tc.expected {
				t.Errorf("Sanitize(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}
