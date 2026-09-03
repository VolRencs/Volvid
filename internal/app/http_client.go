package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type HTTPClientConfig struct {
	Timeout               time.Duration
	DialTimeout           time.Duration
	KeepAlive             time.Duration
	IdleConnTimeout       time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
}

func newTimeoutHTTPClient(timeout time.Duration) *http.Client {
	return newHTTPClient(defaultHTTPClientConfig(timeout))
}

func defaultHTTPClientConfig(timeout time.Duration) HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:               timeout,
		DialTimeout:           defaultDialTimeout,
		KeepAlive:             defaultKeepAlive,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
	}
}

func newDownloadHTTPClient() *http.Client {
	return newHTTPClient(downloadHTTPClientConfig())
}

func downloadHTTPClientConfig() HTTPClientConfig {
	cfg := defaultHTTPClientConfig(0)
	cfg.MaxIdleConns = 32
	cfg.MaxIdleConnsPerHost = 16
	return cfg
}

func newHTTPClient(cfg HTTPClientConfig) *http.Client {
	cfg = normalizeHTTPClientConfig(cfg)
	return &http.Client{
		Timeout:       cfg.Timeout,
		Transport:     buildHTTPTransport(cfg),
		CheckRedirect: safeRedirectPolicy,
	}
}

func safeRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= maxHTTPRedirects {
		return errors.New("stopped after 10 redirects")
	}
	if req == nil || req.URL == nil {
		return errors.New("redirect URL is empty")
	}
	return validateDownloadURL(req.URL.String())
}

func normalizeHTTPClientConfig(cfg HTTPClientConfig) HTTPClientConfig {
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultDialTimeout
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = defaultKeepAlive
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = defaultIdleConnTimeout
	}
	if cfg.ResponseHeaderTimeout <= 0 {
		cfg.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	if cfg.TLSHandshakeTimeout <= 0 {
		cfg.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if cfg.ExpectContinueTimeout <= 0 {
		cfg.ExpectContinueTimeout = defaultExpectContinueTimeout
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 64
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 32
	}
	return cfg
}

func buildHTTPTransport(cfg HTTPClientConfig) *http.Transport {
	transport := cloneDefaultTransport()
	transport.DialContext = (&net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: cfg.KeepAlive,
	}).DialContext
	transport.ForceAttemptHTTP2 = true
	transport.IdleConnTimeout = cfg.IdleConnTimeout
	transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = cfg.TLSHandshakeTimeout
	transport.ExpectContinueTimeout = cfg.ExpectContinueTimeout
	transport.MaxIdleConns = cfg.MaxIdleConns
	transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	return transport
}

func cloneDefaultTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	return base.Clone()
}

func resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
