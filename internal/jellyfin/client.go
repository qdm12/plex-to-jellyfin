package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Settings struct {
	Client  *http.Client
	BaseURL string
	APIKey  string
}

type Client struct {
	client  *http.Client
	baseURL *url.URL
	apiKey  string
}

func New(settings Settings) *Client {
	baseURL, _ := url.Parse(strings.TrimRight(settings.BaseURL, "/"))
	return &Client{
		client:  settings.Client,
		baseURL: baseURL,
		apiKey:  settings.APIKey,
	}
}

func (c *Client) ResolveUserID(ctx context.Context, username string) (userID string, err error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, "/Users")

	var users []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	err = c.doJSON(ctx, http.MethodGet, u.String(), nil, &users)
	if err != nil {
		return "", err
	}

	for _, user := range users {
		if strings.EqualFold(user.Name, username) {
			return user.ID, nil
		}
	}

	return "", fmt.Errorf("jellyfin user %q not found", username)
}

func (c *Client) FetchItems(ctx context.Context, userID string) (items []Item, err error) {
	const pageSize = 200
	var startIndex uint64

	for {
		u := *c.baseURL
		u.Path = path.Join(c.baseURL.Path, "/Users", userID, "Items")
		q := u.Query()
		q.Set("Recursive", "true")
		q.Set("IncludeItemTypes", "Movie,Episode")
		q.Set("Fields", "ProviderIds,Path")
		q.Set("StartIndex", strconv.FormatUint(startIndex, 10))
		q.Set("Limit", strconv.Itoa(pageSize))
		u.RawQuery = q.Encode()

		var response struct {
			Items []struct {
				ID          string            `json:"Id"`
				Type        string            `json:"Type"`
				Name        string            `json:"Name"`
				ProviderIDs map[string]string `json:"ProviderIds"`
				Path        string            `json:"Path"`
			} `json:"Items"`
			TotalRecordCount uint64 `json:"TotalRecordCount"`
		}
		err := c.doJSON(ctx, http.MethodGet, u.String(), nil, &response)
		if err != nil {
			return nil, err
		}

		if len(response.Items) == 0 {
			break
		}

		for _, item := range response.Items {
			items = append(items, Item{
				ID:          item.ID,
				Type:        strings.ToLower(item.Type),
				Name:        item.Name,
				ProviderIDs: item.ProviderIDs,
				Filepath:    item.Path,
			})
			startIndex++
		}
		if startIndex == response.TotalRecordCount {
			break
		}
	}

	return items, nil
}

func (c *Client) UpdateUserData(ctx context.Context, userID, itemID string,
	played bool, positionTicks uint64,
) error {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, "/Users", userID, "Items", itemID, "UserData")

	payload := struct {
		Played                bool   `json:"Played"`
		PlaybackPositionTicks uint64 `json:"PlaybackPositionTicks"`
	}{
		Played:                played,
		PlaybackPositionTicks: positionTicks,
	}

	err := c.doJSON(ctx, http.MethodPost, u.String(), payload, nil)
	if err != nil {
		return fmt.Errorf("updating user data for item %s: %w", itemID, err)
	}

	return nil
}

func (c *Client) doJSON(ctx context.Context, method, address string, payload, responseTarget any) error {
	var requestBody io.Reader
	if payload != nil {
		buffer := bytes.NewBuffer(nil)
		encoder := json.NewEncoder(buffer)
		err := encoder.Encode(payload)
		if err != nil {
			return fmt.Errorf("encoding request payload: %w", err)
		}
		requestBody = buffer
	}

	request, err := http.NewRequestWithContext(ctx, method, address, requestBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	request.Header.Set("X-Emby-Token", c.apiKey)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return fmt.Errorf("unexpected status code %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	if responseTarget == nil {
		return nil
	}

	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(responseTarget)
	if err != nil {
		return fmt.Errorf("decoding JSON response: %w", err)
	}

	return nil
}
