package migrate

import "fmt"

type stats struct {
	plexItems           uint
	matched             uint
	watchedApplied      uint
	unwatchedApplied    uint
	partialApplied      uint
	noJellyfinMatch     uint
	ambiguousJellyfinID uint
}

func (s stats) String() string {
	return fmt.Sprintf(`Plex items: %d
Matched: %d
Watched applied: %d
Unwatched applied: %d
Partial applied: %d
Skipped (no Jellyfin match): %d
Skipped (ambiguous Jellyfin ID): %d`,
		s.plexItems,
		s.matched,
		s.watchedApplied,
		s.unwatchedApplied,
		s.partialApplied,
		s.noJellyfinMatch,
		s.ambiguousJellyfinID,
	)
}
