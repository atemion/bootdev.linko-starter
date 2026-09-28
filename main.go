package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

// var logger = log.New(os.Stderr, "DEBUG: ", log.LstdFlags)

// type closeFunc func() error

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Creates a context to carry request-scoped information. context.Background() creates a base context

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {

	/*
		logger, closer, err := initializeLogger(os.Getenv("LINKO_LOG_FILE"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err) // Print error message to standard error
			return 1
		}
	*/

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	st, err := store.New(dataDir, logger)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to create store: %v", err))
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

	/*
		defer func() {
			if err := closer(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close logger: %v", err) // Print error message to standard error
			}
		}()
	*/

	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Info(fmt.Sprintf("failed to shutdown server: %v", err))
		return 1
	}
	if serverErr != nil {
		logger.Info(fmt.Sprintf("server error: %v", serverErr))
		return 1
	}
	return 0
}

/*
func initializeLogger(logFile string) (*log.Logger, closeFunc, error) {
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}
		bufferedFile := bufio.NewWriterSize(file, 8192)
		multiWriter := io.MultiWriter(os.Stderr, bufferedFile)
		closer := func() error {
			err := bufferedFile.Flush()
			if err != nil {
				return err
			}
			return file.Close()
		}
		return log.New(multiWriter, "", log.LstdFlags), closer, nil
	}
	closer := func() error { return nil }
	return log.New(os.Stderr, "", log.LstdFlags), closer, nil
}
*/
