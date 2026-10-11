package migrate

import (
	"io"
	"testing"

	"github.com/qdm12/log"
	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		libraries []string
		expected  []string
	}{
		"empty_input": {
			libraries: nil,
			expected:  []string{},
		},
		"case_normalized": {
			libraries: []string{"Movies ", "TV Shows"},
			expected:  []string{"movies", "tv shows"},
		},
		"deduplicates_and_sorts": {
			libraries: []string{"Z", "a", "A", "z", " a "},
			expected:  []string{"a", "z"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			migrator := New(nil, nil, nil, "jellyfin-user", testCase.libraries, false)

			assert.Equal(t, testCase.expected, migrator.libraries)
		})
	}
}

func Test_Migrator_matchPlexToJellyfin(t *testing.T) {
	t.Parallel()

	jellyfinItems := []jellyfin.Item{
		{ID: "id1", Type: "movie", Name: "Colorful", ProviderIDs: map[string]string{"Tmdb": "1"}},
	}

	testCases := map[string]struct {
		plexItem             plex.Item
		expectedJellyfinItem jellyfin.Item
		expectedPlayed       bool
		expectedOK           bool
		expectedNoMatchStat  uint
	}{
		"matched_by_provider_id": {
			plexItem: plex.Item{
				Type: "movie", Title: "Whatever",
				ProviderIDs: map[string]string{"tmdb": "1"},
				State:       plex.WatchStateWatched,
			},
			expectedJellyfinItem: jellyfinItems[0],
			expectedPlayed:       true,
			expectedOK:           true,
		},
		"no_match_is_skipped": {
			plexItem: plex.Item{
				Type: "movie", Title: "Cowboy Bebop: The Movie",
				ProviderIDs: map[string]string{"tmdb": "2"},
				State:       plex.WatchStateWatched,
			},
			expectedNoMatchStat: 1,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			logger := log.New(log.SetWriters(io.Discard))
			migrator := New(nil, nil, logger, "jellyfin-user", nil, false)
			var gotStats stats

			jellyfinItem, played, positionTicks, ok := migrator.matchPlexToJellyfin(
				testCase.plexItem, buildMatcher(jellyfinItems), &gotStats)

			assert.Equal(t, testCase.expectedJellyfinItem, jellyfinItem)
			assert.Equal(t, testCase.expectedPlayed, played)
			assert.Zero(t, positionTicks)
			assert.Equal(t, testCase.expectedOK, ok)
			assert.Equal(t, testCase.expectedNoMatchStat, gotStats.noJellyfinMatch)
			if !testCase.expectedOK {
				assert.Zero(t, gotStats.matched)
			}
		})
	}
}
