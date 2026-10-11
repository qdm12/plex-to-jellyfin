package plex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_parseMetadata_playHistory(t *testing.T) {
	t.Parallel()

	type media = []struct {
		Part struct {
			File string `xml:"file,attr"`
		} `xml:"Part"`
	}

	testCases := map[string]struct {
		metadata           xmlVideo
		expectedPlayCount  uint
		expectedLastPlayed time.Time
	}{
		"watched_with_last_viewed": {
			metadata: xmlVideo{
				Type: "movie", ViewCount: 3, LastViewedAt: 1700000000, Media: make(media, 1),
			},
			expectedPlayCount:  3,
			expectedLastPlayed: time.Date(2023, time.November, 14, 22, 13, 20, 0, time.UTC),
		},
		"unwatched_without_last_viewed": {
			metadata: xmlVideo{Type: "movie", Media: make(media, 1)},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			item, ok := parseMetadata(testCase.metadata)

			assert.True(t, ok)
			assert.Equal(t, testCase.expectedPlayCount, item.PlayCount)
			assert.True(t, testCase.expectedLastPlayed.Equal(item.LastPlayed))
		})
	}
}
