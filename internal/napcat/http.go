package napcat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient for OneBot 11 API
type HTTPClient struct {
	url         string
	accessToken string
	client      *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(url, accessToken string) *HTTPClient {
	return &HTTPClient{
		url:         url,
		accessToken: accessToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PostAPI sends an API request via HTTP POST
func (c *HTTPClient) PostAPI(action string, params interface{}) (*APIResponse, error) {
	reqBody := map[string]interface{}{}
	if params != nil {
		// If params is already a map, verify/copy it, or assume it's struct/map
		reqBody = toMap(params)
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s", c.url, action)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		// Sometimes API returns raw data or different format on error
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	return &apiResp, nil
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	// Fallback generic marshal/unmarshal to get map
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
