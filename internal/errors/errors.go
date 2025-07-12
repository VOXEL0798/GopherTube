package errors

import "fmt"

// Error types for better error handling
type (
	SearchError struct {
		Query string
		Err   error
	}

	PlaybackError struct {
		URL string
		Err error
	}

	ConfigError struct {
		Field string
		Err   error
	}

	YTDlpError struct {
		Command string
		Err     error
	}
)

// Error methods
func (e *SearchError) Error() string {
	return fmt.Sprintf("search failed for query '%s': %v", e.Query, e.Err)
}

func (e *SearchError) Unwrap() error {
	return e.Err
}

func (e *PlaybackError) Error() string {
	return fmt.Sprintf("playback failed for URL '%s': %v", e.URL, e.Err)
}

func (e *PlaybackError) Unwrap() error {
	return e.Err
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error for field '%s': %v", e.Field, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

func (e *YTDlpError) Error() string {
	return fmt.Sprintf("yt-dlp command '%s' failed: %v", e.Command, e.Err)
}

func (e *YTDlpError) Unwrap() error {
	return e.Err
}

// Helper functions
func NewSearchError(query string, err error) *SearchError {
	return &SearchError{Query: query, Err: err}
}

func NewPlaybackError(url string, err error) *PlaybackError {
	return &PlaybackError{URL: url, Err: err}
}

func NewConfigError(field string, err error) *ConfigError {
	return &ConfigError{Field: field, Err: err}
}

func NewYTDlpError(command string, err error) *YTDlpError {
	return &YTDlpError{Command: command, Err: err}
}
