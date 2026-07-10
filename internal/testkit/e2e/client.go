// Package e2e contains end-to-end test helpers and fixtures.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// AppClient is an HTTP client for communicating with the app.
type AppClient struct {
	baseURL  string
	password string
	client   *http.Client
	t        *testing.T
}

// NewAppClient creates a new HTTP client for the app.
func NewAppClient(t *testing.T, baseURL, password string) *AppClient {
	t.Helper()

	return &AppClient{
		baseURL:  baseURL,
		password: password,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		t: t,
	}
}

// doRequest performs an HTTP request with automatic Bearer token auth.
func (c *AppClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", c.baseURL, path), bodyReader)
	if err != nil {
		return nil, err
	}

	if c.password != "" {
		req.Header.Set("Authorization", "Bearer "+c.password)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// doRequestWithRetry performs a request with exponential backoff retry.
func (c *AppClient) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, maxRetries int) (*http.Response, error) {
	var lastErr error
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.doRequest(method, path, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue
			}
			backoff *= 2
		}
	}

	return nil, lastErr
}

// GetStatus retrieves the current status from the app.
func (c *AppClient) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequestWithRetry(ctx, "GET", "/status", nil, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return status, nil
}

// ProofOfLife sends a proof of life request with the password.
func (c *AppClient) ProofOfLife(ctx context.Context) (map[string]interface{}, error) {
	payload := map[string]string{"password": c.password}

	resp, err := c.doRequestWithRetry(ctx, "POST", "/alive", payload, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if errMsg, ok := result["error"].(string); ok {
			return nil, fmt.Errorf("proof of life failed: %s", errMsg)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return result, nil
}

// GetHTML retrieves the HTML homepage.
func (c *AppClient) GetHTML(ctx context.Context) (string, error) {
	resp, err := c.doRequestWithRetry(ctx, "GET", "/", nil, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ProofOfLifeWithBadPassword attempts proof of life with an invalid password.
// This is used to test authentication rejection.
func (c *AppClient) ProofOfLifeWithBadPassword(ctx context.Context, badPassword string) (*http.Response, error) {
	payload := map[string]string{"password": badPassword}
	return c.doRequestWithRetry(ctx, "POST", "/alive", payload, 1)
}

// StatusContains checks if a field in the status response matches an expected value.
func StatusContains(status map[string]interface{}, key string, expectedValue interface{}) bool {
	val, ok := status[key]
	if !ok {
		return false
	}

	// Handle boolean comparisons
	if b, ok := expectedValue.(bool); ok {
		if bVal, ok := val.(bool); ok {
			return bVal == b
		}
	}

	// Handle string comparisons
	if s, ok := expectedValue.(string); ok {
		if sVal, ok := val.(string); ok {
			return sVal == s
		}
	}

	return val == expectedValue
}
