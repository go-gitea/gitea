package nessie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/log"

)

type Reference struct {
	Name        string    `json:"name"`
	Hash        string    `json:"hash"`
	Type        string    `json:"type"`
}

// ReferencesResponse represents the Nessie API response structure
type ReferencesResponse struct {
	Token      *string     `json:"token"`
	References []Reference `json:"references"`
	HasMore    bool        `json:"hasMore"`
}

type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL:    setting.Nessie.APIURL,
		authToken:  setting.Nessie.AuthToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetAllReferences(repo string) ([]Reference, error) {
	// TODO: figure out how to handle multiple repositories
	url := fmt.Sprintf("%s/api/v1/trees/", c.baseURL)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response ReferencesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	log.Info("Nessie references: %v", response.References)
	return response.References, nil
} 