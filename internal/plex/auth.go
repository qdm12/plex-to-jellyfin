package plex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FetchToken fetches a Plex token using the provided username, password and one-time code (if any).
func FetchToken(ctx context.Context, client *http.Client,
	username, password, oneTimeCode string,
) (token string, err error) {
	const url = "https://plex.tv/users/sign_in.json"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	request.SetBasicAuth(username, password)
	setPlexHeaders(request.Header)
	if oneTimeCode != "" {
		request.Header.Set("X-Plex-Otp", oneTimeCode)
	}

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("doing sign-in request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		b, _ := io.ReadAll(response.Body)
		responseBody := strings.TrimSpace(string(b))
		return "", fmt.Errorf("unexpected sign-in status code: %d - response %s",
			response.StatusCode, responseBody)
	}

	decoder := json.NewDecoder(response.Body)
	var body struct {
		User struct {
			AuthToken string `json:"authToken"`
		} `json:"user"`
	}
	err = decoder.Decode(&body)
	if err != nil {
		return "", fmt.Errorf("decoding sign-in response: %w", err)
	}

	if body.User.AuthToken == "" {
		return "", errors.New("parsing sign-in response: auth token is empty")
	}

	return body.User.AuthToken, nil
}

func setPlexHeaders(headers http.Header) {
	headers.Set("Accept", "application/json")
	headers.Set("X-Plex-Client-Identifier", "plex-to-jellyfin-web-ui")
	headers.Set("X-Plex-Product", "plex-to-jellyfin")
	headers.Set("X-Plex-Version", "1")
	headers.Set("X-Plex-Platform", "Web")
	headers.Set("X-Plex-Device", "Browser")
}
