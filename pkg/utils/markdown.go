package utils

import "strings"

// EscapeMarkdownV2 escapes all characters that Telegram MarkdownV2 requires
func EscapeMarkdownV2(text string) string {
	specialChars := []string{
		"_", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!",
	}

	for _, ch := range specialChars {
		text = strings.ReplaceAll(text, ch, `\`+ch)
	}

	return text
}
