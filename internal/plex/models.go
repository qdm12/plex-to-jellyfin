package plex

// WatchState indicates whether media is watched, unwatched, or partially watched.
type WatchState string

const (
	WatchStateUnwatched WatchState = "unwatched"
	WatchStatePartial   WatchState = "partial"
	WatchStateWatched   WatchState = "watched"
)

type Section struct {
	Key   string
	Type  string
	Title string
}

// Item is a Plex media item to migrate watch state for.
type Item struct {
	Type          string
	Title         string
	ProviderIDs   map[string]string
	State         WatchState
	PositionTicks uint64
	Filepath      string
}
