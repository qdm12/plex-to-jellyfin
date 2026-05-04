package migrate

import (
	"context"

	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
)

type PlexClient interface {
	FetchSections(ctx context.Context) ([]plex.Section, error)
	FetchSectionItems(ctx context.Context, sectionKey, sectionType string,
	) (items []plex.Item, err error)
}

type JellyfinClient interface {
	ResolveUserID(ctx context.Context, username string) (string, error)
	FetchItems(ctx context.Context, userID string) ([]jellyfin.Item, error)
	UpdateUserData(ctx context.Context, userID string, itemID string,
		played bool, positionTicks uint64) error
}

type Logger interface {
	Debugf(format string, args ...any)
	Info(message string)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}
