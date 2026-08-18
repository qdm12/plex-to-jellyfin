package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// FetchToken fetches a Plex token using the provided username, password and one-time code (if any).
func FetchToken(ctx context.Context, client *http.Client,
	username, password, oneTimeCode string,
) (token string, err error) {
	const signInURL = "https://plex.tv/api/v2/users/signin"

	values := url.Values{}
	values.Set("login", username)
	values.Set("password", password)
	values.Set("rememberMe", "true")
	if oneTimeCode != "" {
		values.Set("verificationCode", oneTimeCode)
	}

	requestBody := strings.NewReader(values.Encode())
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, signInURL, requestBody)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	setPlexHeaders(request.Header)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("reading sign-in response: %w", err)
	}

	var body struct {
		AuthToken string `json:"authToken"`
	}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return "", fmt.Errorf("decoding sign-in response %s: %w", string(b), err)
	}

	if body.AuthToken == "" {
		return "", fmt.Errorf("parsing sign-in response: auth token is empty in: %s", string(b))
	}

	return body.AuthToken, nil
}

func setPlexHeaders(headers http.Header) {
	headers.Set("Accept", "application/json")
	headers.Set("X-Plex-Client-Identifier", "plex-to-jellyfin-web-ui")
	headers.Set("X-Plex-Product", "plex-to-jellyfin")
	headers.Set("X-Plex-Version", "1")
	headers.Set("X-Plex-Platform", "Web")
	headers.Set("X-Plex-Device", "Browser")
}
