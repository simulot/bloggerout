package filename

import (
	"path/filepath"
	"regexp"
	"strings"
)

// reservedNamesWindows is a list of reserved filenames on Windows.
// These are case-insensitive.
var reservedNamesWindows = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

var (
	collapseUnderscores         = regexp.MustCompile(`_{2,}`)
	collapseUnderscoresAndSpace = regexp.MustCompile(`_\s+_`)
)

// Sanitize cleans a string to be a valid and safe filename for
// Windows, macOS, and Linux.
func Sanitize(filename string) string {
	// 1. Replace path separators and illegal characters for Windows/Linux/macOS
	// The replacement character is an underscore.
	replacer := strings.NewReplacer(
		// Common illegal characters for Windows and/or Linux
		"/", "_", "\\", "_", "<", "_", ">", "_", ":", "_", "\"", "_", "|", "_", "?", "_", "*", "_",
		// ASCII control characters (0-31)
		"\x00", "", "\x01", "", "\x02", "", "\x03", "", "\x04", "",
		"\x05", "", "\x06", "", "\x07", "", "\x08", "", "\x09", "",
		"\x0a", "", "\x0b", "", "\x0c", "", "\x0d", "", "\x0e", "",
		"\x0f", "", "\x10", "", "\x11", "", "\x12", "", "\x13", "",
		"\x14", "", "\x15", "", "\x16", "", "\x17", "", "\x18", "",
		"\x19", "", "\x1a", "", "\x1b", "", "\x1c", "", "\x1d", "",
		"\x1e", "", "\x1f", "",
	)
	sanitized := replacer.Replace(filename)

	// 2. Handle Windows reserved filenames.
	// We check the name without the extension.
	baseName := sanitized
	extension := ""
	if dotIndex := strings.LastIndex(sanitized, "."); dotIndex != -1 {
		baseName = sanitized[:dotIndex]
		extension = sanitized[dotIndex:]
	}
	// Check if the base name is a reserved name (case-insensitive)
	if _, isReserved := reservedNamesWindows[strings.ToUpper(baseName)]; isReserved {
		baseName = "_" + baseName
		sanitized = baseName + extension
	}

	// 3. Trim leading/trailing spaces and dots, which are problematic on Windows.
	sanitized = strings.Trim(sanitized, " .")

	// 4. Collapse multiple underscores into one for readability.
	sanitized = collapseUnderscoresAndSpace.ReplaceAllString(sanitized, "_")
	sanitized = collapseUnderscores.ReplaceAllString(sanitized, "_")

	// 5. Ensure the filename is not empty after sanitization.
	sanitized = strings.TrimSuffix(sanitized, "_")

	if sanitized == "" {
		return "unnamed_file"
	}

	// 6. Limit filename length (optional, but good practice).
	// A common limit is 255 characters.
	const maxLength = 255
	if len(sanitized) > maxLength {
		// Try to preserve the extension
		if dotIndex := strings.LastIndex(sanitized, "."); dotIndex != -1 {
			ext := sanitized[dotIndex:]
			base := sanitized[:dotIndex]
			maxBaseLength := maxLength - len(ext)
			if maxBaseLength < 0 {
				maxBaseLength = 0
			}
			if len(base) > maxBaseLength {
				base = base[:maxBaseLength]
			}
			sanitized = base + ext
		} else {
			sanitized = sanitized[:maxLength]
		}
	}

	return sanitized
}

func SanitizePath(p string) string {
	// Split the path into directory and filename
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = Sanitize(parts[i])
	}
	return filepath.Join(p)
}
