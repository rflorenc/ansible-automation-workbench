package platform

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/rflorenc/ansible-automation-workbench/internal/models"
)

// Client is a shared HTTP client used by platform implementations.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a Client from a Connection.
func NewClient(conn *models.Connection) *Client {
	transport := &http.Transport{}
	if conn.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if conn.CACert != "" {
		caCertPool := x509.NewCertPool()
		if caCertPool.AppendCertsFromPEM([]byte(conn.CACert)) {
			transport.TLSClientConfig = &tls.Config{RootCAs: caCertPool}
		}
	}
	return &Client{
		baseURL:  conn.BaseURL(),
		username: conn.Username,
		password: conn.Password,
		httpClient: &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Re-apply basic auth on redirects
				if len(via) > 0 {
					req.SetBasicAuth(conn.Username, conn.Password)
				}
				return nil
			},
		},
	}
}

// paginatedResponse is the standard AWX/AAP paginated response envelope.
type paginatedResponse struct {
	Count   int               `json:"count"`
	Next    *string           `json:"next"`
	Results []json.RawMessage `json:"results"`
}

// Get performs an authenticated GET request and returns the response body.
func (c *Client) Get(path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// GetJSON performs an authenticated GET and unmarshals the response into dest.
func (c *Client) GetJSON(path string, params url.Values, dest interface{}) error {
	body, err := c.Get(path, params)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

// GetAll fetches all pages of a paginated endpoint, returning all results.
func (c *Client) GetAll(path string) ([]models.Resource, error) {
	var all []models.Resource
	currentURL := c.baseURL + path

	for currentURL != "" {
		req, err := http.NewRequest("GET", currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.SetBasicAuth(c.username, c.password)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", currentURL, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s: HTTP %d: %s", currentURL, resp.StatusCode, truncate(string(body), 200))
		}

		var page paginatedResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		for _, raw := range page.Results {
			var res models.Resource
			if err := json.Unmarshal(raw, &res); err != nil {
				return nil, fmt.Errorf("parsing resource: %w", err)
			}
			all = append(all, res)
		}

		if page.Next != nil && *page.Next != "" {
			currentURL = *page.Next
			// If relative URL, make absolute
			if len(currentURL) > 0 && currentURL[0] == '/' {
				currentURL = c.baseURL + currentURL
			}
		} else {
			currentURL = ""
		}
	}
	return all, nil
}

// Post performs an authenticated POST request with a JSON body.
func (c *Client) Post(path string, payload interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, resp.StatusCode, nil
}

// Patch performs an authenticated PATCH request.
func (c *Client) Patch(path string, payload interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("PATCH", c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("PATCH %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("PATCH %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, resp.StatusCode, nil
}

// Delete performs an authenticated DELETE request.
func (c *Client) Delete(path string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 204, resp.StatusCode == 202:
		return nil
	case resp.StatusCode == 404:
		return nil // already gone
	default:
		return fmt.Errorf("DELETE %s: HTTP %d", path, resp.StatusCode)
	}
}

// FindByName searches for a resource by name at the given API path.
func (c *Client) FindByName(path, name string) (models.Resource, error) {
	params := url.Values{"name": {name}}
	body, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	var page paginatedResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if len(page.Results) == 0 {
		return nil, nil
	}
	var res models.Resource
	if err := json.Unmarshal(page.Results[0], &res); err != nil {
		return nil, fmt.Errorf("parsing resource: %w", err)
	}
	return res, nil
}

// FindByUsername searches for a user by username at the given API path.
func (c *Client) FindByUsername(path, username string) (models.Resource, error) {
	params := url.Values{"username": {username}}
	body, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	var page paginatedResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if len(page.Results) == 0 {
		return nil, nil
	}
	var res models.Resource
	if err := json.Unmarshal(page.Results[0], &res); err != nil {
		return nil, fmt.Errorf("parsing resource: %w", err)
	}
	return res, nil
}

// Ping checks connectivity by hitting the API root.
func (c *Client) Ping(apiPath string) error {
	_, err := c.Get(apiPath, nil)
	return err
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
