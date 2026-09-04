package app

import "time"

const (
	defaultDialTimeout           = 30 * time.Second
	defaultKeepAlive             = 30 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = time.Second
	defaultResponseHeaderTimeout = 60 * time.Second
	defaultFileDownloadTimeout   = 2 * time.Hour
	defaultSafeRetryAttempts     = 3
	defaultSafeRetryBackoff      = 250 * time.Millisecond
	maxHTTPRedirects             = 10
	downloadCopyBufferSize       = 1 << 20
	retryBodyDrainLimit          = 1 << 20
	apiClientTimeout             = 8 * time.Second
	manifestFetchTimeout         = 30 * time.Second
	manifestMaxBytes             = 1 << 20
)

const (
	qualityScanTimeout      = 90 * time.Second
	searchTimeout           = 90 * time.Second
	playlistFetchTimeout    = 15 * time.Minute
	maxDetailedQualityURLs  = 5
	maxParallelQualityScans = 6
	runtimeDepsTTL          = 15 * time.Second
	probeCacheTTL           = 10 * time.Minute
)

const (
	versionProbeTimeout  = 1500 * time.Millisecond
	tarCommandTimeout    = 2 * time.Minute
	maxExtractedFileSize = 512 << 20
	folderPickerTimeout  = 2 * time.Minute
)

const (
	processTerminateGrace    = 2 * time.Second
	commandStderrCaptureSize = 8 << 10
	commandStdoutMaxBytes    = 64 << 20
	commandLineBufferSize    = 128 << 10
	commandLineMaxBytes      = 16 << 20
	maxYtdlpErrorLine        = 512
	ffmpegEncodersTimeout    = 3 * time.Second
)

const (
	ytdlpDownloadRetries     = 10
	ytdlpFragmentRetries     = 10
	ytdlpConcurrentFragments = 12
)

const (
	progressEmitInterval = 100 * time.Millisecond
	slotResetDelay       = 300 * time.Millisecond
)
