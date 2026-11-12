package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_requestLogger(t *testing.T) {
	logBuffer := &bytes.Buffer{} // Create a buffer to capture log output

	logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{ // Create a logger that writes to the buffer
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { // Replace the time attribute with a fixed timestamp for testing
			if a.Key == slog.TimeKey {
				return slog.Time(slog.TimeKey, time.Date(2023, 10, 1, 12, 34, 57, 0, time.UTC))
			}
			return a
		},
	}))

	requestLoggerMiddleware := requestLogger(logger)                                  // Create the request logger middleware using the test logger
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}) // Create a dummy handler that does nothing
	loggedHandler := requestLoggerMiddleware(dummyHandler)                            // Wrap the dummy handler with the request logger middleware

	req := httptest.NewRequest("GET", "http://lin.ko/api/stats", nil) // Create a test HTTP request
	rr := httptest.NewRecorder()                                      // Create a ResponseRecorder to capture the response
	loggedHandler.ServeHTTP(rr, req)                                  // Serve the request using the logged handler

	const expectedLogString = `time=2023-10-01T12:34:57.000Z level=INFO msg="Served request" method=GET path=/api/stats client_ip=192.0.2.1:1234` + "\n"
	const expectedStatusCode = http.StatusOK

	// replace the .Skip() call with two checks to verify the log string and status code here
	// If either doesn't match, use t.Errorf to report the failure with a helpful message.

	if logBuffer.String() != expectedLogString { // Check if the log output matches the expected log string
		t.Errorf("Expected log string %q, got %q", expectedLogString, logBuffer.String())
	}
	if rr.Code != expectedStatusCode {
		t.Errorf("Expected status code %d, got %d", expectedStatusCode, rr.Code)
	}
}
