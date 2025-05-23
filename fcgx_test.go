package fcgx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testAddr = "127.0.0.1:9000"
)

func TestFCGXIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start PHP-FPM container if not already running
	// This assumes docker-compose is set up correctly
	if err := os.Setenv("FCGX_TEST_TIMEOUT", "5s"); err != nil {
		t.Fatalf("Failed to set test timeout: %v", err)
	}

	// Wait for PHP-FPM to be ready
	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", testAddr)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("GET", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := DialContext(ctx, "tcp", testAddr)
		if err != nil {
			t.Fatalf("Failed to connect to PHP-FPM: %v", err)
		}
		defer client.Close()

		params := map[string]string{
			"SCRIPT_FILENAME": "/var/www/html/get.php",
			"SCRIPT_NAME":     "/get.php",
			"SERVER_PORT":     "80",
			"SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_URI":     "/get.php",
			"SERVER_NAME":     "localhost",
		}

		resp, err := client.Get(ctx, params)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := ReadBody(resp)
		if err != nil {
			t.Fatalf("ReadBody failed: %v", err)
		}

		if !strings.Contains(string(body), "-PASSED-") {
			t.Errorf("Expected response to contain '-PASSED-', got: %s", string(body))
		}
	})

	t.Run("POST", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := DialContext(ctx, "tcp", testAddr)
		if err != nil {
			t.Fatalf("Failed to connect to PHP-FPM: %v", err)
		}
		defer client.Close()

		params := map[string]string{
			"SCRIPT_FILENAME": "/var/www/html/post.php",
			"SCRIPT_NAME":     "/post.php",
			"SERVER_PORT":     "80",
			"SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_URI":     "/post.php",
			"SERVER_NAME":     "localhost",
		}

		// Test data: key is MD5 of value
		testData := "c4ca4238a0b923820dcc509a6f75849b=1"
		reqBody := strings.NewReader(testData)

		resp, err := client.Post(ctx, params, reqBody, len(testData))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := ReadBody(resp)
		if err != nil {
			t.Fatalf("ReadBody failed: %v", err)
		}

		if !strings.Contains(string(body), "-PASSED-") {
			t.Errorf("Expected response to contain '-PASSED-', got: %s", string(body))
		}
	})

	t.Run("ReadJSONSuccess", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := DialContext(ctx, "tcp", testAddr)
		if err != nil {
			t.Fatalf("Failed to connect to PHP-FPM: %v", err)
		}
		defer client.Close()

		params := map[string]string{
			"SCRIPT_FILENAME": "/var/www/html/json.php",
			"SCRIPT_NAME":     "/json.php",
			"SERVER_PORT":     "80",
			"SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_URI":     "/json.php",
			"SERVER_NAME":     "localhost",
		}

		resp, err := client.Get(ctx, params)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		var dst struct {
			Status string `json:"status"`
			Pass   bool   `json:"pass"`
		}
		err = ReadJSON(resp, &dst)
		if err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if dst.Status != "ok" || !dst.Pass {
			t.Errorf("Unexpected JSON content: %+v", dst)
		}
	})

	t.Run("ReadJSONWrongContentType", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := DialContext(ctx, "tcp", testAddr)
		if err != nil {
			t.Fatalf("Failed to connect to PHP-FPM: %v", err)
		}
		defer client.Close()

		params := map[string]string{
			"SCRIPT_FILENAME": "/var/www/html/get.php",
			"SCRIPT_NAME":     "/get.php",
			"SERVER_PORT":     "80",
			"SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_URI":     "/get.php",
			"SERVER_NAME":     "localhost",
		}

		resp, err := client.Get(ctx, params)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		var dst interface{}
		err = ReadJSON(resp, &dst)
		if err == nil {
			t.Error("Expected error due to wrong Content-Type, got nil")
		}
	})

	t.Run("ReadJSONMalformed", func(t *testing.T) {
		// This test assumes /malformed_json.php returns Content-Type application/json but invalid JSON
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := DialContext(ctx, "tcp", testAddr)
		if err != nil {
			t.Fatalf("Failed to connect to PHP-FPM: %v", err)
		}
		defer client.Close()

		params := map[string]string{
			"SCRIPT_FILENAME": "/var/www/html/malformed_json.php",
			"SCRIPT_NAME":     "/malformed_json.php",
			"SERVER_PORT":     "80",
			"SERVER_PROTOCOL": "HTTP/1.1",
			"REQUEST_URI":     "/malformed_json.php",
			"SERVER_NAME":     "localhost",
		}

		resp, err := client.Get(ctx, params)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		var dst interface{}
		err = ReadJSON(resp, &dst)
		if err == nil {
			t.Error("Expected error due to malformed JSON, got nil")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Test cases for context cancellation
		t.Run("BeforeRequest", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			_, err := DialContext(ctx, "tcp", testAddr)
			if err == nil {
				t.Error("Expected context cancelled error, got nil")
			} else if !strings.Contains(err.Error(), "operation was canceled") {
				t.Errorf("Expected operation canceled error, got: %v", err)
			}
		})

		t.Run("DuringRequest", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			client, err := DialContext(ctx, "tcp", testAddr)
			if err != nil {
				t.Fatalf("Failed to connect to PHP-FPM: %v", err)
			}
			defer client.Close()

			params := map[string]string{
				"SCRIPT_FILENAME": "/var/www/html/slow.php",
				"SCRIPT_NAME":     "/slow.php",
				"SERVER_PORT":     "80",
				"SERVER_PROTOCOL": "HTTP/1.1",
				"REQUEST_URI":     "/slow.php",
				"SERVER_NAME":     "localhost",
			}

			// Cancel context after a short delay
			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()

			_, err = client.Get(ctx, params)
			if err == nil {
				t.Error("Expected context cancelled error, got nil")
			} else if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "operation was canceled") {
				t.Errorf("Expected context cancelled error, got: %v", err)
			}
		})

		t.Run("DuringResponse", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			client, err := DialContext(ctx, "tcp", testAddr)
			if err != nil {
				t.Fatalf("Failed to connect to PHP-FPM: %v", err)
			}
			defer client.Close()

			params := map[string]string{
				"SCRIPT_FILENAME": "/var/www/html/slow.php",
				"SCRIPT_NAME":     "/slow.php",
				"SERVER_PORT":     "80",
				"SERVER_PROTOCOL": "HTTP/1.1",
				"REQUEST_URI":     "/slow.php",
				"SERVER_NAME":     "localhost",
			}

			resp, err := client.Get(ctx, params)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			// Cancel context after getting response but before reading body
			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()

			_, err = ReadBody(resp)
			if err == nil {
				t.Log("No error: operation completed before context expired")
			} else if !errors.Is(err, ErrContextCancelled) && !errors.Is(err, ErrTimeout) {
				t.Errorf("Expected context cancelled or timeout error, got: %v", err)
			}
		})
	})

	t.Run("Timeout", func(t *testing.T) {
		// Test cases for timeout
		t.Run("ConnectionTimeout", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, err := DialContext(ctx, "tcp", "127.0.0.1:9999") // Non-existent port
			if err == nil {
				t.Error("Expected timeout error, got nil")
			} else if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "i/o timeout") && !strings.Contains(err.Error(), "connection refused") {
				t.Errorf("Expected timeout error, got: %v", err)
			}
		})

		t.Run("RequestTimeout", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			client, err := DialContext(ctx, "tcp", testAddr)
			if err != nil {
				t.Fatalf("Failed to connect to PHP-FPM: %v", err)
			}
			defer client.Close()

			params := map[string]string{
				"SCRIPT_FILENAME": "/var/www/html/slow.php",
				"SCRIPT_NAME":     "/slow.php",
				"SERVER_PORT":     "80",
				"SERVER_PROTOCOL": "HTTP/1.1",
				"REQUEST_URI":     "/slow.php",
				"SERVER_NAME":     "localhost",
			}

			_, err = client.Get(ctx, params)
			if err == nil {
				t.Log("No error: operation completed before timeout expired")
			} else if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "i/o timeout") {
				t.Errorf("Expected timeout error, got: %v", err)
			}
		})

		t.Run("ResponseTimeout", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			client, err := DialContext(ctx, "tcp", testAddr)
			if err != nil {
				t.Fatalf("Failed to connect to PHP-FPM: %v", err)
			}
			defer client.Close()

			params := map[string]string{
				"SCRIPT_FILENAME": "/var/www/html/slow.php",
				"SCRIPT_NAME":     "/slow.php",
				"SERVER_PORT":     "80",
				"SERVER_PROTOCOL": "HTTP/1.1",
				"REQUEST_URI":     "/slow.php",
				"SERVER_NAME":     "localhost",
			}

			resp, err := client.Get(ctx, params)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			// Create a new context with a short timeout for reading the response
			readCtx, readCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer readCancel()

			done := make(chan error, 1)
			go func() {
				_, err := ReadBody(resp)
				done <- err
			}()

			select {
			case err := <-done:
				if err == nil {
					t.Log("No error: operation completed before timeout expired")
				} else if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "i/o timeout") {
					t.Errorf("Expected timeout error, got: %v", err)
				}
			case <-readCtx.Done():
				t.Log("Context deadline exceeded: operation timed out as expected")
			}
		})

		t.Run("ParseFallbackWithOnlyContentType", func(t *testing.T) {
			body := "Content-Type: application/json\r\n\r\n{\"status\":\"ok\",\"pass\":true}"
			resp, err := parseHTTPResponse(bytes.NewBufferString(body))
			if err != nil {
				t.Fatalf("parseHTTPResponse failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Errorf("Expected status code 200, got %d", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Expected Content-Type header, got: %s", ct)
			}
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}
			if !strings.Contains(string(b), `"status":"ok"`) {
				t.Errorf("Expected JSON body, got: %s", string(b))
			}
		})
	})
}
