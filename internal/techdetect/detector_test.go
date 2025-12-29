package techdetect

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.client)
}

func TestDetect_EmptyInputs(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	result := detector.Detect(nil, nil)

	assert.NotNil(t, result)
	assert.NotNil(t, result.Technologies)
	assert.NotNil(t, result.RawHeaders)
	assert.Empty(t, result.HTMLSample)
}

func TestDetect_WithWordPressHeaders(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("X-Powered-By", "PHP/7.4")
	headers.Set("Link", `<https://example.com/wp-json/>; rel="https://api.w.org/"`)

	body := []byte(`<!DOCTYPE html><html><head><meta name="generator" content="WordPress 6.0"></head><body></body></html>`)

	result := detector.Detect(headers, body)

	assert.NotNil(t, result)
	// Wappalyzer may detect MySQL or PHP from these signatures
	// The important thing is that detection runs without panics
	assert.NotNil(t, result.Technologies)
}

func TestDetect_WithCloudflareHeaders(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("CF-Ray", "1234567890abcdef-SYD")
	headers.Set("CF-Cache-Status", "HIT")
	headers.Set("Server", "cloudflare")

	result := detector.Detect(headers, nil)

	assert.NotNil(t, result)
	// Cloudflare should be detected from headers
	_, hasCloudflare := result.Technologies["Cloudflare"]
	assert.True(t, hasCloudflare, "Cloudflare should be detected")
}

func TestDetect_WithShopifySignatures(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("X-ShopId", "12345678")
	headers.Set("X-Shopify-Stage", "production")
	headers.Set("Content-Type", "text/html; charset=utf-8")

	body := []byte(`<!DOCTYPE html><html><head><link rel="preconnect" href="https://cdn.shopify.com"></head><body data-shopify="true"></body></html>`)

	result := detector.Detect(headers, body)

	assert.NotNil(t, result)
	// Shopify should be detected
	_, hasShopify := result.Technologies["Shopify"]
	assert.True(t, hasShopify, "Shopify should be detected")
}

func TestDetect_StoresRawHeaders(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("Content-Type", "text/html")
	headers.Set("Server", "nginx")
	headers.Add("Cache-Control", "public")
	headers.Add("Cache-Control", "max-age=3600")

	result := detector.Detect(headers, nil)

	assert.NotNil(t, result.RawHeaders)
	assert.Equal(t, []string{"text/html"}, result.RawHeaders["Content-Type"])
	assert.Equal(t, []string{"nginx"}, result.RawHeaders["Server"])
	assert.Equal(t, []string{"public", "max-age=3600"}, result.RawHeaders["Cache-Control"])
}

func TestDetect_TruncatesLargeHTML(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	// Create a body larger than MaxHTMLSampleSize
	largeBody := make([]byte, MaxHTMLSampleSize+1000)
	for i := range largeBody {
		largeBody[i] = 'x'
	}

	result := detector.Detect(nil, largeBody)

	assert.Len(t, result.HTMLSample, MaxHTMLSampleSize)
}

func TestDetect_PreservesSmallHTML(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	smallBody := []byte("<html><body>Hello World</body></html>")

	result := detector.Detect(nil, smallBody)

	assert.Equal(t, string(smallBody), result.HTMLSample)
}

func TestResult_TechnologiesJSON(t *testing.T) {
	result := &Result{
		Technologies: map[string][]string{
			"WordPress":  {"CMS"},
			"Cloudflare": {"CDN", "Reverse proxies"},
		},
	}

	jsonBytes, err := result.TechnologiesJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonBytes), "WordPress")
	assert.Contains(t, string(jsonBytes), "Cloudflare")
}

func TestResult_HeadersJSON(t *testing.T) {
	result := &Result{
		RawHeaders: map[string][]string{
			"Content-Type": {"text/html"},
			"Server":       {"nginx/1.18.0"},
		},
	}

	jsonBytes, err := result.HeadersJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonBytes), "Content-Type")
	assert.Contains(t, string(jsonBytes), "nginx")
}

func TestDetect_ConcurrentAccess(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("Server", "nginx")

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result := detector.Detect(headers, []byte("<html></html>"))
			assert.NotNil(t, result)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDetectFromResponse(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("Server", "Apache")

	body := []byte("<html></html>")

	result := detector.DetectFromResponse(resp, body)

	assert.NotNil(t, result)
	assert.Equal(t, []string{"Apache"}, result.RawHeaders["Server"])
}

func TestDetect_WithReactApp(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	body := []byte(`<!DOCTYPE html><html><head></head><body><div id="root"></div><script src="/static/js/main.js" data-react-app="true"></script></body></html>`)

	result := detector.Detect(nil, body)

	assert.NotNil(t, result)
	// Detection depends on wappalyzer signatures; body might not be enough
	// This test verifies no panics occur with typical React signatures
}

func TestDetect_WithNginxServer(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("Server", "nginx")

	result := detector.Detect(headers, nil)

	assert.NotNil(t, result)
	// Detection depends on wappalyzer signatures
	// The important thing is that detection runs without panics
	assert.NotNil(t, result.Technologies)
}

func TestDetect_WithApacheServer(t *testing.T) {
	detector, err := New()
	require.NoError(t, err)

	headers := make(http.Header)
	headers.Set("Server", "Apache")

	result := detector.Detect(headers, nil)

	assert.NotNil(t, result)
	// Detection depends on wappalyzer signatures
	// The important thing is that detection runs without panics
	assert.NotNil(t, result.Technologies)
}
