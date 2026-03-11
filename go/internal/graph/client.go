package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const graphBaseURL = "https://graph.microsoft.com"

type Client struct {
	token      string
	httpClient *http.Client
}

type responseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type TodoList struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type ChecklistItem struct {
	DisplayName string `json:"displayName"`
	IsChecked   bool   `json:"isChecked"`
}

type ItemBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type DateTimeTimeZone struct {
	DateTime string `json:"dateTime"`
}

type Task struct {
	Title          string            `json:"title"`
	Status         string            `json:"status"`
	Body           ItemBody          `json:"body"`
	DueDateTime    *DateTimeTimeZone `json:"dueDateTime"`
	ChecklistItems []ChecklistItem   `json:"checklistItems"`
}

type page[T any] struct {
	Value    []T    `json:"value"`
	NextLink string `json:"@odata.nextLink"`
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Lists(ctx context.Context) ([]TodoList, error) {
	return getAllPages[TodoList](ctx, c, "/v1.0/me/todo/lists")
}

func (c *Client) Tasks(ctx context.Context, listID string) ([]Task, error) {
	path := fmt.Sprintf("/v1.0/me/todo/lists/%s/tasks?$expand=checklistItems", listID)
	return getAllPages[Task](ctx, c, path)
}

func getAllPages[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var items []T
	nextPath := path

	for nextPath != "" {
		var current page[T]
		if err := c.get(ctx, nextPath, &current); err != nil {
			return nil, err
		}
		items = append(items, current.Value...)

		nextPath = current.NextLink
		nextPath = strings.TrimPrefix(nextPath, graphBaseURL)
	}

	return items, nil
}

func (c *Client) get(ctx context.Context, path string, target any) error {
	url := path
	if strings.HasPrefix(path, "/") {
		url = graphBaseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var graphErr responseError
	if err := json.Unmarshal(body, &graphErr); err == nil && graphErr.Error.Code != "" {
		return fmt.Errorf("Graph API error: %s - %s", graphErr.Error.Code, graphErr.Error.Message)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}
