package converter

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// HTMLToMarkdown extracts readable markdown content from raw HTML.
func HTMLToMarkdown(html string) (string, error) {
	markdown, err := md.ConvertString(html)
	if err != nil {
		return "", fmt.Errorf("html-to-markdown: %w", err)
	}
	return strings.TrimSpace(markdown), nil
}
