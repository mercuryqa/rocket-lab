package testutil

import (
	"net/http"
	"time"
)

// NewHTTPClient создаёт HTTP клиент с разумными таймаутами для тестирования.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}
