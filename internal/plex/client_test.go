package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Client_FetchSectionItems(t *testing.T) {
	t.Parallel()

	const responseBody = `<MediaContainer>
  <Video type="movie" title="Watched" viewCount="3" lastViewedAt="1700000000" guid="plex://movie/abc">
    <Guid id="tmdb://1"/>
    <Media><Part file="/movies/watched.mkv"/></Media>
  </Video>
  <Video type="movie" title="Partial" viewOffset="5000">
    <Guid id="tmdb://2"/>
    <Media><Part file="/movies/partial.mkv"/></Media>
  </Video>
  <Video type="movie" title="Unwatched">
    <Guid id="tmdb://3"/>
    <Media><Part file="/movies/unwatched.mkv"/></Media>
  </Video>
  <Video type="clip" title="Ignored">
    <Media><Part file="/clips/ignored.mkv"/></Media>
  </Video>
</MediaContainer>`

	testCases := map[string]struct {
		sectionType  string
		expectedPath string
	}{
		"movie_section": {
			sectionType:  "movie",
			expectedPath: "/library/sections/1/all",
		},
		"show_section": {
			sectionType:  "show",
			expectedPath: "/library/sections/1/allLeaves",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var receivedPath, receivedToken string
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, request *http.Request) {
					receivedPath = request.URL.Path
					receivedToken = request.URL.Query().Get("X-Plex-Token")
					_, err := writer.Write([]byte(responseBody))
					assert.NoError(t, err)
				}))
			t.Cleanup(server.Close)

			client := New(Settings{Client: server.Client(), BaseURL: server.URL, Token: "token"})

			items, err := client.FetchSectionItems(t.Context(), "1", testCase.sectionType)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedPath, receivedPath)
			assert.Equal(t, "token", receivedToken)
			require.Len(t, items, 3)

			assert.Equal(t, "Watched", items[0].Title)
			assert.Equal(t, WatchStateWatched, items[0].State)
			assert.Equal(t, uint(3), items[0].PlayCount)
			assert.True(t, time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC).Equal(items[0].LastPlayed))

			assert.Equal(t, "Partial", items[1].Title)
			assert.Equal(t, WatchStatePartial, items[1].State)
			assert.Equal(t, uint64(50000000), items[1].PositionTicks)
			assert.Zero(t, items[1].PlayCount)
			assert.True(t, items[1].LastPlayed.IsZero())

			assert.Equal(t, "Unwatched", items[2].Title)
			assert.Equal(t, WatchStateUnwatched, items[2].State)
			assert.Zero(t, items[2].PlayCount)
			assert.True(t, items[2].LastPlayed.IsZero())
		})
	}
}
