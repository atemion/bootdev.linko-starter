package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/linkoerr"
	"boot.dev/linko/internal/store"
	pkgerr "github.com/pkg/errors"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Creates a context to carry request-scoped information. context.Background() creates a base context

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	logFilePath := os.Getenv("LINKO_LOG_FILE")
	if logFilePath == "" {
		logFilePath = "./linko.access.log"
	}

	status := run(ctx, cancel, *httpPort, *dataDir, logFilePath)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string, logFilePath string) int {

	debugHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttr,
	})

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.Error("failed to open log file", "error", err)
		return 1
	}
	defer logFile.Close()
	infoHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceAttr,
	})

	logger := slog.New(slog.NewMultiHandler(
		debugHandler,
		infoHandler,
	))

	st, err := store.New(dataDir, logger)
	if err != nil {
		logger.Error("failed to create store", "error", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, logger)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()

	<-ctx.Done()                                                                    // Wait for the context to be canceled (e.g., on SIGINT or SIGTERM)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Create a new context with a timeout for the shutdown process
	defer cancel()                                                                  // Ensure that the cancel function is called to release resources associated with the context

	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown server", "error", err)
		return 1
	}
	if serverErr != nil {
		logger.Error("server error", "error", serverErr)
		return 1
	}
	return 0
}

type stackTracer interface { // Define an interface for errors that can provide a stack trace
	error
	StackTrace() pkgerr.StackTrace // Method to retrieve the stack trace
}

type multiError interface {
	error
	Unwrap() []error
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error) // Check if the value is of type error
		if !ok {
			return a
		}

		if me, ok := errors.AsType[multiError](err); ok { // Check if the error implements the multiError interface
			var errAttrs []slog.Attr
			for i, err := range me.Unwrap() {
				errAttrs = append(errAttrs, slog.GroupAttrs(fmt.Sprintf("error_%d", i+1), errorAttrs(err)...)) // Recursively extract attributes for each error in the multiError
			}
			return slog.GroupAttrs("errors", errAttrs...) // Return a group of attributes for the multiple errors
		}
		return slog.GroupAttrs("error", errorAttrs(err)...) // Return a group of attributes for the error
	}
	return a
}

func errorAttrs(err error) []slog.Attr {
	attrs := []slog.Attr{{
		Key:   "message",
		Value: slog.StringValue(err.Error()),
	}}
	attrs = append(attrs, linkoerr.Attrs(err)...)
	if stackErr, ok := errors.AsType[stackTracer](err); ok {
		attrs = append(attrs, slog.Attr{
			Key:   "stack_trace",
			Value: slog.StringValue(fmt.Sprintf("%+v", stackErr.StackTrace())),
		})
	}
	return attrs
}
