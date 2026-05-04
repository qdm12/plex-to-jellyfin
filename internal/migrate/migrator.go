package migrate

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
)

type Migrator struct {
	plexClient     PlexClient
	jellyfinClient JellyfinClient
	logger         Logger
	jellyfinUser   string
	libraries      []string
	dryRun         bool
}

func New(plex PlexClient, jellyfin JellyfinClient, logger Logger,
	jellyfinUser string, libraries []string, dryRun bool,
) *Migrator {
	slices.Sort(libraries)
	libraries = slices.Compact(libraries)
	return &Migrator{
		plexClient:     plex,
		jellyfinClient: jellyfin,
		logger:         logger,
		jellyfinUser:   jellyfinUser,
		libraries:      libraries,
		dryRun:         dryRun,
	}
}

func (m Migrator) Run(ctx context.Context) error {
	plexItems, err := m.fetchPlexData(ctx)
	if err != nil {
		return fmt.Errorf("fetching Plex data: %w", err)
	}

	m.logger.Info("resolving Jellyfin user...")
	jellyfinUserID, err := m.jellyfinClient.ResolveUserID(ctx, m.jellyfinUser)
	if err != nil {
		return fmt.Errorf("resolving Jellyfin user %q: %w", m.jellyfinUser, err)
	}

	m.logger.Info("fetching Jellyfin items...")
	jellyfinItems, err := m.jellyfinClient.FetchItems(ctx, jellyfinUserID)
	if err != nil {
		return fmt.Errorf("fetching Jellyfin items: %w", err)
	}

	m.logger.Infof("found %d items from Plex and %d items from Jellyfin",
		len(plexItems), len(jellyfinItems))

	matcher := buildMatcher(jellyfinItems)
	stats := stats{
		plexItems: uint(len(plexItems)),
	}
	for _, plexItem := range plexItems {
		jellyfinItem, played, positionTicks, ok := m.matchPlexToJellyfin(plexItem, matcher, &stats)
		if !ok {
			continue
		}

		m.logger.Debugf("matched Plex item %q with Jellyfin item %q", plexItem.Title, jellyfinItem.Name)

		if m.dryRun {
			continue
		}

		err = m.jellyfinClient.UpdateUserData(ctx, jellyfinUserID, jellyfinItem.ID, played, positionTicks)
		if err != nil {
			return fmt.Errorf("updating %q (%s): %w", jellyfinItem.Name, jellyfinItem.ID, err)
		}
	}

	m.logger.Infof("migration complete, statistics:\n%s", stats)

	if m.dryRun {
		m.logger.Info("Dry run enabled: no updates were sent to Jellyfin")
	}

	return nil
}

func (m Migrator) fetchPlexData(ctx context.Context) (items []plex.Item, err error) {
	m.logger.Info("fetching Plex library sections...")
	sections, err := m.plexClient.FetchSections(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching Plex sections: %w", err)
	}

	for _, section := range sections {
		switch {
		case len(m.libraries) > 0 && !slices.Contains(m.libraries, strings.ToLower(section.Title)):
			m.logger.Infof("skipping Plex section %q because it is not in the list of libraries to migrate",
				section.Title)
			continue
		case section.Type != "movie" && section.Type != "show":
			m.logger.Warnf("skipping Plex section %q because its type %q is not supported",
				section.Title, section.Type)
			continue
		}
		m.logger.Infof("reading Plex section %q...", section.Title)

		sectionItems, err := m.plexClient.FetchSectionItems(ctx, section.Key, section.Type)
		if err != nil {
			return nil, fmt.Errorf("fetching items for section %q: %w", section.Title, err)
		}
		m.logger.Infof("found %d items from Plex section %q", len(sectionItems), section.Title)
		items = append(items, sectionItems...)
	}

	return items, nil
}

func (m Migrator) matchPlexToJellyfin(plexItem plex.Item, matcher itemMatcher, stats *stats) (
	jellyfinItem jellyfin.Item, played bool, positionTicks uint64, ok bool,
) {
	matches := matcher.find(plexItem.Type, plexItem.ProviderIDs)
	if len(matches) != 1 {
		match, ambiguousWarnings, ok := m.matchByFallbackStrategies(plexItem, matcher)
		if !ok {
			if len(ambiguousWarnings) > 0 || len(matches) > 1 {
				stats.ambiguousJellyfinID++
				for _, warning := range ambiguousWarnings {
					m.logger.Warnf(warning)
				}
			} else {
				stats.noJellyfinMatch++
			}
			m.logger.Warnf("no Jellyfin match for Plex item %q", plexItem.Title)
		}
		matches = []jellyfin.Item{match}
	}

	stats.matched++

	switch plexItem.State {
	case plex.WatchStateWatched:
		played = true
		stats.watchedApplied++
	case plex.WatchStatePartial:
		positionTicks = plexItem.PositionTicks
		stats.partialApplied++
	case plex.WatchStateUnwatched:
		stats.unwatchedApplied++
	default:
		m.logger.Errorf("unknown watch state for Plex item %q: %s",
			plexItem.Title, plexItem.State)
		return jellyfin.Item{}, false, 0, false
	}

	return matches[0], played, positionTicks, true
}

func (m Migrator) matchByFallbackStrategies(plexItem plex.Item, matcher itemMatcher) (
	match jellyfin.Item, ambiguousWarnings []string, ok bool,
) {
	matches := matcher.findByExactFilepath(plexItem.Type, plexItem.Filepath)
	switch {
	case len(matches) == 1:
		m.logger.Infof("fallback matched Plex item %q by exact filepath with Jellyfin item %q (path %s)",
			plexItem.Title, matches[0].Name, plexItem.Filepath)
		return matches[0], nil, true
	case len(matches) > 1:
		ambiguousWarnings = append(ambiguousWarnings,
			fmt.Sprintf("ambiguous filepath match for Plex item %q: %d items",
				plexItem.Title, len(matches)))
	}

	matches = matcher.findApproxByTitle(plexItem.Type, plexItem.Title)
	switch {
	case len(matches) == 1:
		m.logger.Infof("fallback matched Plex item %q by approximate title with Jellyfin item %q",
			plexItem.Title, matches[0].Name)
		return matches[0], ambiguousWarnings, true
	case len(matches) > 1:
		ambiguousWarnings = append(ambiguousWarnings,
			fmt.Sprintf("ambiguous title match for Plex item %q: %d items",
				plexItem.Title, len(matches)))
	}

	return jellyfin.Item{}, ambiguousWarnings, false
}

type itemMatcher struct {
	typeToIDToItems       map[string]map[string][]jellyfin.Item
	typeToItems           map[string][]jellyfin.Item
	typeToFilepathToItems map[string]map[string][]jellyfin.Item
}

func buildMatcher(items []jellyfin.Item) itemMatcher {
	matcher := itemMatcher{
		typeToIDToItems:       make(map[string]map[string][]jellyfin.Item),
		typeToItems:           make(map[string][]jellyfin.Item),
		typeToFilepathToItems: make(map[string]map[string][]jellyfin.Item),
	}
	for _, item := range items {
		typeKey := strings.ToLower(item.Type)
		matcher.typeToItems[typeKey] = append(matcher.typeToItems[typeKey], item)

		typeMap, ok := matcher.typeToIDToItems[typeKey]
		if !ok {
			typeMap = make(map[string][]jellyfin.Item)
			matcher.typeToIDToItems[typeKey] = typeMap
		}
		for provider, id := range item.ProviderIDs {
			lookupKey := normalizeProviderID(provider, id)
			if lookupKey == "" {
				continue
			}
			typeMap[lookupKey] = append(typeMap[lookupKey], item)
		}

		filepathToItems, ok := matcher.typeToFilepathToItems[typeKey]
		if !ok {
			filepathToItems = make(map[string][]jellyfin.Item)
			matcher.typeToFilepathToItems[typeKey] = filepathToItems
		}
		filepathToItems[item.Filepath] = append(filepathToItems[item.Filepath], item)
	}
	return matcher
}

func (m itemMatcher) find(itemType string, providerIDs map[string]string) []jellyfin.Item {
	idToItems := m.typeToIDToItems[strings.ToLower(itemType)]

	itemIDToItem := make(map[string]jellyfin.Item)
	for provider, id := range providerIDs {
		lookupKey := normalizeProviderID(provider, id)
		if lookupKey == "" {
			continue
		}
		for _, item := range idToItems[lookupKey] {
			itemIDToItem[item.ID] = item
		}
	}

	return slices.Collect(maps.Values(itemIDToItem))
}

func normalizeProviderID(provider, id string) (lookupKey string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	id = strings.ToLower(strings.TrimSpace(id))
	if provider == "" || id == "" {
		return ""
	}

	switch {
	case strings.Contains(provider, "imdb"):
		return "imdb:" + id
	case strings.Contains(provider, "tmdb"):
		return "tmdb:" + id
	case strings.Contains(provider, "tvdb"):
		return "tvdb:" + id
	default:
		return ""
	}
}

func (m itemMatcher) findApproxByTitle(itemType, title string) []jellyfin.Item {
	target := normalizeTitle(title)
	if target == "" {
		return nil
	}

	items := m.typeToItems[strings.ToLower(itemType)]
	if len(items) == 0 {
		return nil
	}

	const minTitleSimilarity = 0.78
	const scoreEpsilon = 0.02

	bestScore := 0.0
	bestItems := make([]jellyfin.Item, 0, 1)
	for _, item := range items {
		score := titleSimilarity(target, normalizeTitle(item.Name))
		if score < minTitleSimilarity {
			continue
		}

		scoreDiff := math.Abs(score - bestScore)
		if score > bestScore+scoreEpsilon {
			bestScore = score
			bestItems = []jellyfin.Item{item}
			continue
		}

		if scoreDiff <= scoreEpsilon {
			bestItems = append(bestItems, item)
		}
	}

	return bestItems
}

func (m itemMatcher) findByExactFilepath(itemType, filepath string) []jellyfin.Item {
	if filepath == "" {
		return nil
	}
	filepathToItems := m.typeToFilepathToItems[strings.ToLower(itemType)]
	return filepathToItems[filepath]
}

func normalizeTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(len(value))
	lastWasSpace := false
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			lastWasSpace = false
			continue
		}

		if lastWasSpace {
			continue
		}
		builder.WriteByte(' ')
		lastWasSpace = true
	}

	return strings.TrimSpace(builder.String())
}

func titleSimilarity(left, right string) float64 {
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 1
	}

	containmentScore, contained := containmentSimilarity(left, right)
	if contained {
		return containmentScore
	}

	return diceSimilarity(left, right)
}

func containmentSimilarity(left, right string) (score float64, contained bool) {
	if !strings.Contains(left, right) && !strings.Contains(right, left) {
		return 0, false
	}

	const containmentBaseScore = 0.85
	const containmentScoreScale = 0.15

	shortestLength := min(len(left), len(right))
	longestLength := max(len(left), len(right))
	coverage := float64(shortestLength) / float64(longestLength)
	return containmentBaseScore + (containmentScoreScale * coverage), true
}

func diceSimilarity(left, right string) float64 {
	const multiplier = 2

	leftBigrams := makeBigrams(left)
	rightBigrams := makeBigrams(right)
	if len(leftBigrams) == 0 || len(rightBigrams) == 0 {
		return 0
	}

	intersectionCount := 0
	for bigram, leftCount := range leftBigrams {
		rightCount, ok := rightBigrams[bigram]
		if !ok {
			continue
		}
		if leftCount < rightCount {
			intersectionCount += leftCount
			continue
		}
		intersectionCount += rightCount
	}

	leftCount := 0
	for _, count := range leftBigrams {
		leftCount += count
	}
	rightCount := 0
	for _, count := range rightBigrams {
		rightCount += count
	}

	return (multiplier * float64(intersectionCount)) / float64(leftCount+rightCount)
}

func makeBigrams(value string) map[string]int {
	const minRuneCount = 2

	runes := []rune(value)
	if len(runes) < minRuneCount {
		return nil
	}

	bigrams := make(map[string]int, len(runes)-1)
	for i := range len(runes) - 1 {
		bigram := string([]rune{runes[i], runes[i+1]})
		bigrams[bigram]++
	}
	return bigrams
}
