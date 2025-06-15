package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewWebFetcher(t *testing.T) {
	wf := NewWebFetcher()

	if wf == nil {
		t.Fatal("NewWebFetcher returned nil")
	}

	if wf.client == nil {
		t.Error("WebFetcher client is nil")
	}

	if wf.maxSize != 10*1024*1024 {
		t.Errorf("Expected maxSize to be 10MB, got %d", wf.maxSize)
	}

	if wf.timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", wf.timeout)
	}

	if wf.userAgent != "Bazinga/1.0 AI Assistant" {
		t.Errorf("Expected userAgent to be 'Bazinga/1.0 AI Assistant', got %s", wf.userAgent)
	}
}

func TestWebFetcher_Fetch_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("User-Agent") != "Bazinga/1.0 AI Assistant" {
			t.Errorf("Expected User-Agent header, got %s", r.Header.Get("User-Agent"))
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<html>
				<head><title>Test Page</title></head>
				<body>
					<main>
						<h1>Main Content</h1>
						<p>This is the main content of the page.</p>
					</main>
					<nav>Navigation menu</nav>
					<script>console.log('test');</script>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	content, err := wf.Fetch(ctx, server.URL)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Check that content was cleaned
	if strings.Contains(content, "<script>") {
		t.Error("Content should not contain script tags")
	}

	if strings.Contains(content, "Navigation menu") {
		t.Error("Content should not contain navigation content")
	}

	if !strings.Contains(content, "Main Content") {
		t.Error("Content should contain main content")
	}

	if !strings.Contains(content, "This is the main content") {
		t.Error("Content should contain paragraph text")
	}
}

func TestWebFetcher_Fetch_InvalidURL(t *testing.T) {
	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, "://invalid-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("Expected 'invalid URL' error, got: %v", err)
	}
}

func TestWebFetcher_Fetch_UnsupportedScheme(t *testing.T) {
	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, "ftp://example.com")
	if err == nil {
		t.Error("Expected error for unsupported scheme")
	}

	if !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Errorf("Expected 'unsupported URL scheme' error, got: %v", err)
	}
}

func TestWebFetcher_Fetch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for HTTP 404")
	}

	if !strings.Contains(err.Error(), "HTTP error 404") {
		t.Errorf("Expected 'HTTP error 404', got: %v", err)
	}
}

func TestWebFetcher_Fetch_UnsupportedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for unsupported content type")
	}

	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("Expected 'unsupported content type' error, got: %v", err)
	}
}

func TestWebFetcher_Fetch_SizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		// Write content larger than maxSize
		largeContent := strings.Repeat("a", 11*1024*1024) // 11MB
		_, _ = w.Write([]byte(largeContent))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for content too large")
	}

	if !strings.Contains(err.Error(), "response too large") {
		t.Errorf("Expected 'response too large' error, got: %v", err)
	}
}

func TestWebFetcher_Fetch_PlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("This is plain text content."))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	content, err := wf.Fetch(ctx, server.URL)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	expected := "This is plain text content."
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

func TestIsTextContent(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"text/html", true},
		{"text/plain", true},
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/xhtml+xml", true},
		{"text/html; charset=utf-8", true},
		{"image/png", false},
		{"video/mp4", false},
		{"application/pdf", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := isTextContent(tt.contentType)
			if result != tt.expected {
				t.Errorf("Expected %t for content type '%s', got %t", tt.expected, tt.contentType, result)
			}
		})
	}
}

func TestWebFetcher_CleanHTML(t *testing.T) {
	wf := NewWebFetcher()

	tests := []struct {
		name        string
		input       string
		contains    []string
		notContains []string
	}{
		{
			name: "Remove scripts and styles",
			input: `
				<html>
					<head>
						<script>alert('test');</script>
						<style>body { color: red; }</style>
					</head>
					<body>
						<p>Main content</p>
					</body>
				</html>
			`,
			contains:    []string{"Main content"},
			notContains: []string{"alert('test')", "color: red"},
		},
		{
			name: "Extract main content",
			input: `
				<html>
					<body>
						<nav>Navigation</nav>
						<main>
							<h1>Article Title</h1>
							<p>Article content here.</p>
						</main>
						<footer>Footer content</footer>
					</body>
				</html>
			`,
			contains:    []string{"Article Title", "Article content here"},
			notContains: []string{"Navigation", "Footer content"},
		},
		{
			name: "Extract article content",
			input: `
				<html>
					<body>
						<header>Site header</header>
						<article>
							<h2>Blog Post</h2>
							<p>This is a blog post content.</p>
						</article>
						<aside>Sidebar content</aside>
					</body>
				</html>
			`,
			contains:    []string{"Blog Post", "This is a blog post content"},
			notContains: []string{"Site header", "Sidebar content"},
		},
		{
			name: "Fallback to body when no main content",
			input: `
				<html>
					<body>
						<div>
							<p>Some content without main tags.</p>
							<p>Another paragraph.</p>
						</div>
						<nav>Should be removed</nav>
					</body>
				</html>
			`,
			contains:    []string{"Some content without main tags", "Another paragraph"},
			notContains: []string{"Should be removed"},
		},
		{
			name: "Remove duplicate lines",
			input: `
				<html>
					<body>
						<main>
							<p>Same content</p>
							<p>Same content</p>
							<p>Different content</p>
						</main>
					</body>
				</html>
			`,
			contains:    []string{"Same content", "Different content"},
			notContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wf.cleanHTML(tt.input)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', but it didn't. Result: %s", expected, result)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected result to NOT contain '%s', but it did. Result: %s", notExpected, result)
				}
			}
		})
	}
}

func TestWebFetcher_BasicCleanHTML(t *testing.T) {
	wf := NewWebFetcher()

	input := `
		<html>
			<head>
				<script>alert('test');</script>
				<style>body { color: red; }</style>
			</head>
			<body>
				<p>Main <strong>content</strong> here.</p>
				<!-- This is a comment -->
				<div>Another paragraph.</div>
			</body>
		</html>
	`

	result := wf.basicCleanHTML(input)

	// Should contain text content
	if !strings.Contains(result, "Main content here") {
		t.Error("Expected result to contain 'Main content here'")
	}

	if !strings.Contains(result, "Another paragraph") {
		t.Error("Expected result to contain 'Another paragraph'")
	}

	// Should not contain script, style, or comments
	if strings.Contains(result, "alert('test')") {
		t.Error("Result should not contain script content")
	}

	if strings.Contains(result, "color: red") {
		t.Error("Result should not contain style content")
	}

	if strings.Contains(result, "This is a comment") {
		t.Error("Result should not contain comment content")
	}
}

func TestWebFetcher_Fetch_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Test</body></html>"))
	}))
	defer server.Close()

	wf := NewWebFetcher()

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := wf.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestWebFetcher_Fetch_RedirectLimit(t *testing.T) {
	redirectCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if redirectCount <= 15 { // More than the 10 redirect limit
			w.Header().Set("Location", server.URL+"/redirect")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Final content</body></html>"))
	}))
	defer server.Close()

	wf := NewWebFetcher()
	ctx := context.Background()

	_, err := wf.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Expected error due to too many redirects")
	}

	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("Expected 'too many redirects' error, got: %v", err)
	}
}
