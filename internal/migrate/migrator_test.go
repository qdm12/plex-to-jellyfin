package migrate

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/qdm12/log"
	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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

func Test_Migrator_Run(t *testing.T) {
	t.Parallel()

	errUpdate := errors.New("update error")
	lastPlayed := time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC)
	jellyfinItems := []jellyfin.Item{
		{ID: "id1", Type: "movie", Name: "Colorful", ProviderIDs: map[string]string{"Tmdb": "1"}},
	}
	sections := []plex.Section{{Key: "1", Type: "movie", Title: "Movies"}}

	testCases := map[string]struct {
		plexItem       plex.Item
		dryRun         bool
		updateExpected bool
		played         bool
		positionTicks  uint64
		playCount      uint
		lastPlayed     time.Time
		updateErr      error
		errMessage     string
	}{
		"watched_with_play_history": {
			plexItem: plex.Item{
				Type: "movie", Title: "Colorful", ProviderIDs: map[string]string{"tmdb": "1"},
				State: plex.WatchStateWatched, PlayCount: 3, LastPlayed: lastPlayed,
			},
			updateExpected: true,
			played:         true,
			playCount:      3,
			lastPlayed:     lastPlayed,
		},
		"partial_without_play_history": {
			plexItem: plex.Item{
				Type: "movie", Title: "Colorful", ProviderIDs: map[string]string{"tmdb": "1"},
				State: plex.WatchStatePartial, PositionTicks: 5000,
			},
			updateExpected: true,
			positionTicks:  5000,
		},
		"dry_run_does_not_update": {
			plexItem: plex.Item{
				Type: "movie", Title: "Colorful", ProviderIDs: map[string]string{"tmdb": "1"},
				State: plex.WatchStateWatched, PlayCount: 3, LastPlayed: lastPlayed,
			},
			dryRun: true,
		},
		"unmatched_item_does_not_update": {
			plexItem: plex.Item{
				Type: "movie", Title: "Cowboy Bebop: The Movie", ProviderIDs: map[string]string{"tmdb": "2"},
				State: plex.WatchStateWatched, PlayCount: 1, LastPlayed: lastPlayed,
			},
		},
		"update_error": {
			plexItem: plex.Item{
				Type: "movie", Title: "Colorful", ProviderIDs: map[string]string{"tmdb": "1"},
				State: plex.WatchStateWatched, PlayCount: 3, LastPlayed: lastPlayed,
			},
			updateExpected: true,
			played:         true,
			playCount:      3,
			lastPlayed:     lastPlayed,
			updateErr:      errUpdate,
			errMessage:     `updating "Colorful" (id1): update error`,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			ctx := t.Context()

			plexClient := NewMockPlexClient(ctrl)
			plexClient.EXPECT().FetchSections(ctx).Return(sections, nil)
			plexClient.EXPECT().FetchSectionItems(ctx, "1", "movie").
				Return([]plex.Item{testCase.plexItem}, nil)

			jellyfinClient := NewMockJellyfinClient(ctrl)
			jellyfinClient.EXPECT().ResolveUserID(ctx, "jellyfin-user").Return("user-id", nil)
			jellyfinClient.EXPECT().FetchItems(ctx, "user-id").Return(jellyfinItems, nil)
			if testCase.updateExpected {
				jellyfinClient.EXPECT().UpdateUserData(ctx, "user-id", "id1",
					testCase.played, testCase.positionTicks, testCase.playCount, testCase.lastPlayed).
					Return(testCase.updateErr)
			}

			logger := log.New(log.SetWriters(io.Discard))
			migrator := New(plexClient, jellyfinClient, logger, "jellyfin-user", nil, testCase.dryRun)

			err := migrator.Run(ctx)

			if testCase.errMessage != "" {
				assert.ErrorIs(t, err, errUpdate)
				assert.ErrorContains(t, err, testCase.errMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
