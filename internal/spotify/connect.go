package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/steipete/spogo/internal/cookies"
)

type ConnectOptions struct {
	Source   cookies.Source
	Market   string
	Language string
	Device   string
	Timeout  time.Duration
}

type ConnectClient struct {
	source       cookies.Source
	market       string
	language     string
	device       string
	client       *http.Client
	session      *connectSession
	hashes       *hashResolver
	webMu        sync.Mutex
	web          *Client
	searchURL    string
	searchClient *http.Client
}

func NewConnectClient(opts ConnectOptions) (*ConnectClient, error) {
	if opts.Source == nil {
		return nil, errors.New("cookie source required")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	session := &connectSession{
		source: opts.Source,
		client: httpClient,
	}
	return &ConnectClient{
		source:   opts.Source,
		market:   opts.Market,
		language: opts.Language,
		device:   opts.Device,
		client:   httpClient,
		session:  session,
		hashes:   newHashResolver(httpClient, session),
	}, nil
}

func (c *ConnectClient) Search(ctx context.Context, kind, query string, limit, offset int) (SearchResult, error) {
	return c.search(ctx, kind, query, limit, offset)
}

func (c *ConnectClient) GetTrack(ctx context.Context, id string) (Item, error) {
	return c.trackInfo(ctx, id)
}

func (c *ConnectClient) GetAlbum(ctx context.Context, id string) (Item, error) {
	return c.albumInfo(ctx, id)
}

func (c *ConnectClient) GetArtist(ctx context.Context, id string) (Item, error) {
	return c.artistInfo(ctx, id)
}

func (c *ConnectClient) GetPlaylist(ctx context.Context, id string) (Item, error) {
	return c.playlistInfo(ctx, id)
}

func (c *ConnectClient) GetShow(ctx context.Context, id string) (Item, error) {
	return c.showInfo(ctx, id)
}

func (c *ConnectClient) GetEpisode(ctx context.Context, id string) (Item, error) {
	return c.episodeInfo(ctx, id)
}

func (c *ConnectClient) Playback(ctx context.Context) (PlaybackStatus, error) {
	return c.playback(ctx)
}

func (c *ConnectClient) Play(ctx context.Context, uri string) error {
	return c.play(ctx, uri)
}

func (c *ConnectClient) Pause(ctx context.Context) error {
	return c.pause(ctx)
}

func (c *ConnectClient) Next(ctx context.Context) error {
	return c.next(ctx)
}

func (c *ConnectClient) Previous(ctx context.Context) error {
	return c.previous(ctx)
}

func (c *ConnectClient) Seek(ctx context.Context, positionMS int) error {
	return c.seek(ctx, positionMS)
}

func (c *ConnectClient) Volume(ctx context.Context, volume int) error {
	return c.volume(ctx, volume)
}

func (c *ConnectClient) Shuffle(ctx context.Context, enabled bool) error {
	return c.shuffle(ctx, enabled)
}

func (c *ConnectClient) Repeat(ctx context.Context, mode string) error {
	return c.repeat(ctx, mode)
}

func (c *ConnectClient) Devices(ctx context.Context) ([]Device, error) {
	return c.devices(ctx)
}

func (c *ConnectClient) Transfer(ctx context.Context, deviceID string) error {
	return c.transfer(ctx, deviceID)
}

func (c *ConnectClient) QueueAdd(ctx context.Context, uri string) error {
	return c.queueAdd(ctx, uri)
}

func (c *ConnectClient) Queue(ctx context.Context) (Queue, error) {
	return c.queue(ctx)
}

func (c *ConnectClient) LibraryTracks(ctx context.Context, limit, offset int) ([]Item, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	payload, err := c.graphQL(ctx, "fetchLibraryTracks", map[string]any{
		"offset": offset,
		"limit":  limit,
	})
	if err != nil {
		return nil, 0, err
	}
	container, ok := getMap(payload, "data", "me", "library", "tracks")
	if !ok {
		// Best-effort fallback: extract anything that looks like a track.
		items := collectItemsByKind(payload, "track")
		return items, len(items), nil
	}
	items := extractItemsFromContainer(container, "track")
	total := getInt(container, "totalLength")
	if total == 0 {
		total = getInt(container, "totalCount")
	}
	if total == 0 {
		total = len(items)
	}
	return items, total, nil
}

func (c *ConnectClient) LibraryAlbums(ctx context.Context, limit, offset int) ([]Item, int, error) {
	return c.libraryV3List(ctx, "Albums", "album", limit, offset)
}

func (c *ConnectClient) LibraryModify(ctx context.Context, path string, ids []string, method string) error {
	kind := ""
	switch {
	case strings.Contains(path, "tracks"):
		kind = "track"
	case strings.Contains(path, "albums"):
		kind = "album"
	default:
		return fmt.Errorf("%w: unsupported library path %q", ErrUnsupported, path)
	}
	return c.libraryModifyByKind(ctx, kind, ids, method)
}

func (c *ConnectClient) FollowArtists(ctx context.Context, ids []string, method string) error {
	return c.libraryModifyByKind(ctx, "artist", ids, method)
}

func (c *ConnectClient) FollowedArtists(ctx context.Context, limit int, after string) ([]Item, int, string, error) {
	if after != "" {
		// The Web API uses `after` cursor pagination. WebPlayer library uses offset pagination.
		return nil, 0, "", fmt.Errorf("%w: after cursor not supported in connect engine", ErrUnsupported)
	}
	items, total, err := c.libraryV3List(ctx, "Artists", "artist", limit, 0)
	return items, total, "", err
}

func (c *ConnectClient) Playlists(ctx context.Context, limit, offset int) ([]Item, int, error) {
	return c.libraryV3List(ctx, "Playlists", "playlist", limit, offset)
}

func (c *ConnectClient) RecentlyPlayed(ctx context.Context, limit int) ([]RecentItem, error) {
	if limit <= 0 {
		limit = 50
	}
	payload, err := c.graphQL(ctx, "recents", map[string]any{
		"uris":   []string{"spotify:list:recents:page"},
		"offset": 0,
		"limit":  limit,
	})
	if err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return nil, errors.New("missing data")
	}
	lists, _ := data["lists"].([]any)
	if len(lists) == 0 {
		return nil, nil
	}
	list0, _ := lists[0].(map[string]any)
	if list0 == nil {
		return nil, errors.New("invalid lists payload")
	}
	itemsContainer, _ := list0["items"].(map[string]any)
	if itemsContainer == nil {
		return nil, nil
	}
	rawItems, _ := itemsContainer["items"].([]any)
	out := make([]RecentItem, 0, len(rawItems))
	for _, raw := range rawItems {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entity := entry["entity"]
		item, ok := extractItem(entity, "track")
		if !ok {
			continue
		}
		playedAt := ""
		// recents uses a date object in addedAt (no time-of-day).
		if addedAt, ok := entry["addedAt"].(map[string]any); ok {
			year := getInt(addedAt, "year")
			month := getInt(addedAt, "month")
			day := getInt(addedAt, "day")
			if year > 0 && month > 0 && day > 0 {
				playedAt = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			}
		}
		out = append(out, RecentItem{
			Track:    item,
			PlayedAt: playedAt,
		})
	}
	return out, nil
}

func (c *ConnectClient) libraryModifyByKind(ctx context.Context, kind string, ids []string, method string) error {
	if len(ids) == 0 {
		return nil
	}
	op := ""
	switch method {
	case http.MethodPut:
		op = "addToLibrary"
	case http.MethodDelete:
		op = "removeFromLibrary"
	default:
		return fmt.Errorf("%w: unsupported library method %q", ErrUnsupported, method)
	}
	uris := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if strings.HasPrefix(id, "spotify:") {
			uris = append(uris, id)
			continue
		}
		uris = append(uris, "spotify:"+kind+":"+id)
	}
	if len(uris) == 0 {
		return nil
	}
	_, err := c.graphQL(ctx, op, map[string]any{
		"libraryItemUris": uris,
	})
	return err
}

func (c *ConnectClient) libraryV3List(ctx context.Context, filterID string, kind string, limit, offset int) ([]Item, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	payload, err := c.graphQL(ctx, "libraryV3", map[string]any{
		"filters":         []string{filterID},
		"order":           "Recents",
		"textFilter":      nil,
		"features":        []string{},
		"limit":           limit,
		"offset":          offset,
		"flatten":         false,
		"expandedFolders": nil,
		"folderUri":       nil,
	})
	if err != nil {
		return nil, 0, err
	}
	lib, ok := getMap(payload, "data", "me", "libraryV3")
	if !ok {
		return nil, 0, errors.New("missing libraryV3")
	}
	rawItems, _ := lib["items"].([]any)
	out := make([]Item, 0, len(rawItems))
	for _, raw := range rawItems {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		item, ok := extractItem(entry["item"], kind)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	total := getInt(lib, "totalCount")
	if total == 0 {
		total = len(out)
	}
	return out, total, nil
}

func (c *ConnectClient) PlaylistTracks(ctx context.Context, id string, limit, offset int) ([]Item, int, error) {
	if limit <= 0 {
		limit = 100
	}

	variables := map[string]any{
		"uri":    "spotify:playlist:" + id,
		"offset": offset,
		"limit":  limit,
	}

	payload, err := c.graphQL(ctx, "fetchPlaylistContents", variables)
	if err != nil {
		return nil, 0, err
	}

	items, total := extractPlaylistTracksFromPayload(payload)
	return items, total, nil
}

func (c *ConnectClient) CreatePlaylist(ctx context.Context, name string, public, collaborative bool) (Item, error) {
	if c.session == nil {
		return Item{}, errors.New("connect session not initialized")
	}
	auth, err := c.session.auth(ctx)
	if err != nil {
		return Item{}, err
	}

	createURL := "https://spclient.wg.spotify.com/playlist/v2/playlist"
	createPayload := map[string]any{
		"ops": []map[string]any{
			{
				"kind": "UPDATE_LIST_ATTRIBUTES",
				"updateListAttributes": map[string]any{
					"newAttributes": map[string]any{
						"values": map[string]any{
							"name": name,
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(createPayload)
	req, _ := http.NewRequestWithContext(ctx, "POST", createURL, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("Client-Token", auth.ClientToken)
	req.Header.Set("spotify-app-version", auth.ClientVersion)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("app-platform", "WebPlayer")

	resp, err := c.client.Do(req)
	if err != nil {
		return Item{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Item{}, fmt.Errorf("create playlist failed: %d", resp.StatusCode)
	}

	var result struct {
		URI string `json:"uri"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	id := strings.TrimPrefix(result.URI, "spotify:playlist:")
	return Item{
		ID:   id,
		URI:  result.URI,
		Name: name,
		Type: "playlist",
		URL:  "https://open.spotify.com/playlist/" + id,
	}, nil
}

func (c *ConnectClient) AddTracks(ctx context.Context, playlistID string, uris []string) error {
	if len(uris) == 0 {
		return nil
	}

	// Ensure URIs are in correct format
	formattedURIs := make([]string, len(uris))
	for i, uri := range uris {
		if !strings.HasPrefix(uri, "spotify:") {
			formattedURIs[i] = "spotify:track:" + uri
		} else {
			formattedURIs[i] = uri
		}
	}

	variables := map[string]any{
		"playlistItemUris": formattedURIs,
		"playlistUri":      "spotify:playlist:" + playlistID,
		"newPosition": map[string]any{
			"moveType": "BOTTOM_OF_PLAYLIST",
			"fromUid":  nil,
		},
	}

	_, err := c.graphQL(ctx, "addToPlaylist", variables)
	return err
}

func (c *ConnectClient) RemoveTracks(ctx context.Context, playlistID string, uris []string) error {
	if len(uris) == 0 {
		return nil
	}

	// Normalize URIs to spotify:track:XXX format
	normalizedURIs := make(map[string]bool)
	for _, uri := range uris {
		if !strings.HasPrefix(uri, "spotify:") {
			uri = "spotify:track:" + uri
		}
		normalizedURIs[uri] = true
	}

	// Fetch playlist contents to get UIDs for each track
	uidsToRemove, err := c.getPlaylistTrackUIDs(ctx, playlistID, normalizedURIs)
	if err != nil {
		return err
	}

	if len(uidsToRemove) == 0 {
		return nil
	}

	variables := map[string]any{
		"playlistUri": "spotify:playlist:" + playlistID,
		"uids":        uidsToRemove,
	}

	_, err = c.graphQL(ctx, "removeFromPlaylist", variables)
	return err
}

func (c *ConnectClient) getPlaylistTrackUIDs(ctx context.Context, playlistID string, targetURIs map[string]bool) ([]string, error) {
	var uids []string
	offset := 0
	limit := 100

	for {
		variables := map[string]any{
			"uri":    "spotify:playlist:" + playlistID,
			"offset": offset,
			"limit":  limit,
		}

		payload, err := c.graphQL(ctx, "fetchPlaylistContents", variables)
		if err != nil {
			return nil, err
		}

		items := extractPlaylistItems(payload)
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			uid, _ := itemMap["uid"].(string)
			itemData, _ := itemMap["itemV2"].(map[string]any)
			if itemData == nil {
				continue
			}
			data, _ := itemData["data"].(map[string]any)
			if data == nil {
				continue
			}
			trackURI, _ := data["uri"].(string)

			if uid != "" && trackURI != "" && targetURIs[trackURI] {
				uids = append(uids, uid)
			}
		}

		if len(items) < limit {
			break
		}
		offset += limit
	}

	return uids, nil
}

func extractPlaylistItems(payload map[string]any) []any {
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return nil
	}
	playlistV2, _ := data["playlistV2"].(map[string]any)
	if playlistV2 == nil {
		return nil
	}
	content, _ := playlistV2["content"].(map[string]any)
	if content == nil {
		return nil
	}
	items, _ := content["items"].([]any)
	return items
}

func extractPlaylistTracksFromPayload(payload map[string]any) ([]Item, int) {
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return nil, 0
	}
	playlistV2, _ := data["playlistV2"].(map[string]any)
	if playlistV2 == nil {
		return nil, 0
	}
	content, _ := playlistV2["content"].(map[string]any)
	if content == nil {
		return nil, 0
	}

	total := 0
	if t, ok := content["totalCount"].(float64); ok {
		total = int(t)
	}

	rawItems, _ := content["items"].([]any)
	items := make([]Item, 0, len(rawItems))

	for _, raw := range rawItems {
		itemMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		itemV2, _ := itemMap["itemV2"].(map[string]any)
		if itemV2 == nil {
			continue
		}

		trackData, _ := itemV2["data"].(map[string]any)
		if trackData == nil {
			continue
		}

		uri, _ := trackData["uri"].(string)
		if uri == "" || !strings.HasPrefix(uri, "spotify:track:") {
			continue
		}

		name, _ := trackData["name"].(string)

		var artists []string
		if artistsData, ok := trackData["artists"].(map[string]any); ok {
			if artistItems, ok := artistsData["items"].([]any); ok {
				for _, a := range artistItems {
					if artistMap, ok := a.(map[string]any); ok {
						if profile, ok := artistMap["profile"].(map[string]any); ok {
							if artistName, ok := profile["name"].(string); ok {
								artists = append(artists, artistName)
							}
						}
					}
				}
			}
		}

		var album string
		if albumData, ok := trackData["albumOfTrack"].(map[string]any); ok {
			if albumName, ok := albumData["name"].(string); ok {
				album = albumName
			}
		}

		var durationMS int
		if playability, ok := trackData["playability"].(map[string]any); ok {
			if dur, ok := playability["playable"].(bool); ok && dur {
				if d, ok := trackData["duration"].(map[string]any); ok {
					if ms, ok := d["totalMilliseconds"].(float64); ok {
						durationMS = int(ms)
					}
				}
			}
		}

		id := strings.TrimPrefix(uri, "spotify:track:")
		items = append(items, Item{
			ID:         id,
			URI:        uri,
			Name:       name,
			Type:       "track",
			URL:        "https://open.spotify.com/track/" + id,
			Artists:    artists,
			Album:      album,
			DurationMS: durationMS,
		})
	}

	return items, total
}
