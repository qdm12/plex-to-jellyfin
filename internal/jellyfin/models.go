package jellyfin

// Item is a Jellyfin media item with provider IDs.
type Item struct {
	ID          string
	Type        string
	Name        string
	ProviderIDs map[string]string
	Filepath    string
}
