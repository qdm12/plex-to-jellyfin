package jellyfin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Client_UpdateUserData(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		statusCode           int
		played               bool
		positionTicks        uint64
		playCount            uint
		lastPlayed           time.Time
		expectedBody         string
		expectedErrorMessage string
	}{
		"watched_with_last_played": {
			statusCode: http.StatusOK,
			played:     true,
			playCount:  3,
			lastPlayed: time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC),
			expectedBody: `{"Played":true,"PlaybackPositionTicks":0,"PlayCount":3,` +
				`"LastPlayedDate":"2023-11-14T22:13:20Z"}`,
		},
		"partial_without_last_played": {
			statusCode:    http.StatusOK,
			positionTicks: 5000,
			expectedBody:  `{"Played":false,"PlaybackPositionTicks":5000,"PlayCount":0}`,
		},
		"unexpected_status_code": {
			statusCode:           http.StatusInternalServerError,
			expectedBody:         `{"Played":false,"PlaybackPositionTicks":0,"PlayCount":0}`,
			expectedErrorMessage: "updating user data for item item1: unexpected status code 500",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			type receivedRequest struct {
				method        string
				path          string
				authorization string
				contentType   string
				body          string
			}
			var received receivedRequest

			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, request *http.Request) {
					body, err := io.ReadAll(request.Body)
					assert.NoError(t, err)
					received = receivedRequest{
						method:        request.Method,
						path:          request.URL.Path,
						authorization: request.Header.Get("Authorization"),
						contentType:   request.Header.Get("Content-Type"),
						body:          string(body),
					}
					writer.WriteHeader(testCase.statusCode)
				}))
			t.Cleanup(server.Close)

			client := New(Settings{Client: server.Client(), BaseURL: server.URL + "/", APIKey: "secret"})

			err := client.UpdateUserData(t.Context(), "user1", "item1",
				testCase.played, testCase.positionTicks, testCase.playCount, testCase.lastPlayed)

			if testCase.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, testCase.expectedErrorMessage)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, http.MethodPost, received.method)
			assert.Equal(t, "/Users/user1/Items/item1/UserData", received.path)
			assert.Equal(t, `MediaBrowser Token="secret"`, received.authorization)
			assert.Equal(t, "application/json", received.contentType)
			assert.JSONEq(t, testCase.expectedBody, received.body)
		})
	}
}
