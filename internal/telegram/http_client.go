package telegram

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = time.Minute

// NewHTTPClient returns a Telegram-specific HTTP client.
// When proxyURL is empty, it falls back to a direct client.
func NewHTTPClient(proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if proxyURL == "" {
		transport.Proxy = nil
		return &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		}, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse TELEGRAM_PROXY: %w", err)
	}

	if parsedURL.Scheme != "socks5" && parsedURL.Scheme != "socks5h" {
		return nil, fmt.Errorf("unsupported TELEGRAM_PROXY scheme %q", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("TELEGRAM_PROXY must include host:port")
	}

	transport.Proxy = http.ProxyURL(parsedURL)

	return &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}, nil
}
