package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type githubIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

func createGitHubIssue(ctx context.Context, token, repo, title, body string, labels []string) error {
	payload, err := json.Marshal(githubIssueRequest{
		Title:  title,
		Body:   body,
		Labels: labels,
	})
	if err != nil {
		return fmt.Errorf("marshal issue payload: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	return nil
}
