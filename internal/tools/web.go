package tools

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WebFetcher handles web content fetching
type WebFetcher struct {
	client    *http.Client
	maxSize   int64 // Maximum response size in bytes
	timeout   time.Duration
	userAgent string
}

// NewWebFetcher creates a new web fetcher
func NewWebFetcher() *WebFetcher {
	return &WebFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Limit redirects to prevent infinite loops
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxSize:   10 * 1024 * 1024, // 10MB max
		timeout:   30 * time.Second,
		userAgent: "Bazinga/1.0 AI Assistant",
	}
}

// Fetch retrieves content from a URL
func (wf *WebFetcher) Fetch(ctx context.Context, targetURL string) (string, error) {
	// Validate and normalize URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure we have a scheme
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
		targetURL = parsedURL.String()
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s (only http and https are allowed)", parsedURL.Scheme)
	}

	loggy.Debug("WebFetcher: fetching URL", "url", targetURL)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent and accept headers
	req.Header.Set("User-Agent", wf.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Make the request
	resp, err := wf.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !isTextContent(contentType) {
		return "", fmt.Errorf("unsupported content type: %s (only text content is supported)", contentType)
	}

	// Read response with size limit
	limitedReader := io.LimitReader(resp.Body, wf.maxSize)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check if we hit the size limit
	if int64(len(content)) >= wf.maxSize {
		return "", fmt.Errorf("response too large (exceeded %d bytes)", wf.maxSize)
	}

	contentStr := string(content)

	// Basic content cleaning for HTML
	if strings.Contains(contentType, "html") {
		contentStr = wf.cleanHTML(contentStr)
	}

	loggy.Info("WebFetcher: successfully fetched content",
		"url", targetURL,
		"size", len(contentStr),
		"content_type", contentType)

	return contentStr, nil
}

// isTextContent checks if the content type is text-based
func isTextContent(contentType string) bool {
	contentType = strings.ToLower(contentType)
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/xhtml+xml",
	}

	for _, textType := range textTypes {
		if strings.Contains(contentType, textType) {
			return true
		}
	}

	return false
}

// cleanHTML performs HTML cleaning to extract readable text using goquery
func (wf *WebFetcher) cleanHTML(html string) string {
	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		loggy.Debug("WebFetcher: failed to parse HTML with goquery, falling back to basic cleaning", "error", err)
		return wf.basicCleanHTML(html)
	}

	// Remove script, style, and other non-content elements
	doc.Find("script, style, noscript, nav, header, footer, aside").Remove()

	// Remove comments (goquery handles this automatically)

	// Extract text content from main content areas
	var textParts []string

	// Try to find main content areas first
	mainSelectors := []string{
		"main", "article", "[role='main']", ".main-content", "#main-content",
		".content", "#content", ".post-content", ".entry-content",
	}

	foundMainContent := false
	for _, selector := range mainSelectors {
		if selection := doc.Find(selector); selection.Length() > 0 {
			selection.Each(func(i int, s *goquery.Selection) {
				if text := strings.TrimSpace(s.Text()); text != "" {
					textParts = append(textParts, text)
					foundMainContent = true
				}
			})
			if foundMainContent {
				break
			}
		}
	}

	// If no main content found, extract from body but skip navigation/sidebar elements
	if !foundMainContent {
		doc.Find("body").Each(func(i int, s *goquery.Selection) {
			// Remove navigation, sidebar, and other non-content elements
			s.Find("nav, .nav, .navigation, .sidebar, .menu, .header, .footer, .ads, .advertisement").Remove()

			if text := strings.TrimSpace(s.Text()); text != "" {
				textParts = append(textParts, text)
			}
		})
	}

	// Join and clean up the text
	fullText := strings.Join(textParts, "\n\n")

	// Clean up excessive whitespace
	lines := strings.Split(fullText, "\n")
	var cleanLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and very short lines that are likely navigation/UI elements
		if len(line) > 2 {
			cleanLines = append(cleanLines, line)
		}
	}

	// Remove duplicate consecutive lines
	var finalLines []string
	var lastLine string

	for _, line := range cleanLines {
		if line != lastLine {
			finalLines = append(finalLines, line)
			lastLine = line
		}
	}

	return strings.Join(finalLines, "\n")
}

// basicCleanHTML is a fallback method using the original manual parsing
func (wf *WebFetcher) basicCleanHTML(html string) string {
	// Remove script and style tags
	html = removeTagContent(html, "script")
	html = removeTagContent(html, "style")
	html = removeTagContent(html, "noscript")

	// Remove HTML comments
	html = removeComments(html)

	// Remove common HTML tags but keep their content
	tags := []string{"html", "head", "body", "div", "span", "p", "br", "hr", "img", "a", "strong", "em", "b", "i", "u", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "table", "tr", "td", "th", "thead", "tbody", "nav", "header", "footer", "section", "article", "aside"}
	for _, tag := range tags {
		html = removeTags(html, tag)
	}

	// Clean up whitespace
	lines := strings.Split(html, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// removeTagContent removes tags and their content
func removeTagContent(content, tag string) string {
	for {
		start := strings.Index(strings.ToLower(content), "<"+tag)
		if start == -1 {
			break
		}

		end := strings.Index(strings.ToLower(content[start:]), "</"+tag+">")
		if end == -1 {
			break
		}

		end += start + len("</"+tag+">")
		content = content[:start] + content[end:]
	}
	return content
}

// removeTags removes HTML tags but keeps their content
func removeTags(content, tag string) string {
	// Remove opening tags
	content = strings.ReplaceAll(content, "<"+tag+">", "")
	content = strings.ReplaceAll(content, "<"+strings.ToUpper(tag)+">", "")

	// Remove closing tags
	content = strings.ReplaceAll(content, "</"+tag+">", "")
	content = strings.ReplaceAll(content, "</"+strings.ToUpper(tag)+">", "")

	// Remove tags with attributes (basic approach)
	for {
		start := strings.Index(strings.ToLower(content), "<"+tag+" ")
		if start == -1 {
			break
		}

		end := strings.Index(content[start:], ">")
		if end == -1 {
			break
		}

		content = content[:start] + content[start+end+1:]
	}

	return content
}

// removeComments removes HTML comments
func removeComments(content string) string {
	for {
		start := strings.Index(content, "<!--")
		if start == -1 {
			break
		}

		end := strings.Index(content[start:], "-->")
		if end == -1 {
			break
		}

		end += start + 3
		content = content[:start] + content[end:]
	}
	return content
}
