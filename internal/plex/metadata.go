package plex

import (
	"strconv"
	"strings"
)

func parseMetadata(metadata xmlVideo) (item Item, ok bool) {
	if metadata.Type != "movie" && metadata.Type != "episode" {
		return Item{}, false
	}

	state := WatchStateUnwatched
	var positionTicks uint64
	switch {
	case metadata.ViewCount > 0:
		state = WatchStateWatched
	case metadata.ViewOffset > 0:
		state = WatchStatePartial
		const ticksPerMillisecond = 10000
		positionTicks = metadata.ViewOffset * ticksPerMillisecond
	}

	providerIDs := make(map[string]string, len(metadata.Guids)+1)
	if metadata.GUID != "" {
		key, value, ok := parseProviderID(metadata.GUID)
		if ok {
			providerIDs[key] = value
		}
	}
	for _, guid := range metadata.Guids {
		key, value, ok := parseProviderID(guid.ID)
		if !ok {
			return Item{}, false
		}
		providerIDs[key] = value
	}

	return Item{
		Type:          metadata.Type,
		Title:         metadata.Title,
		ProviderIDs:   providerIDs,
		State:         state,
		PositionTicks: positionTicks,
		Filepath:      metadata.Media[0].Part.File,
	}, true
}

func parseProviderID(raw string) (key, value string, ok bool) {
	parts := strings.SplitN(raw, "://", 2) //nolint:mnd
	if len(parts) != 2 {                   //nolint:mnd
		return "", "", false
	}

	key = strings.ToLower(strings.TrimSpace(parts[0]))
	value = strings.TrimSpace(parts[1])

	switch {
	case strings.Contains(key, "imdb"):
		key = "imdb"
	case strings.Contains(key, "tmdb"):
		key = "tmdb"
	case strings.Contains(key, "tvdb"):
		key = "tvdb"
	default:
		return "", "", false
	}

	value = strings.TrimPrefix(value, "movie/")
	value = strings.TrimPrefix(value, "show/")
	value = strings.TrimPrefix(value, "episode/")
	value = strings.Trim(value, "/")
	if value == "" {
		return "", "", false
	}

	if key == "tmdb" || key == "tvdb" {
		_, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", "", false
		}
	}

	return key, value, true
}
