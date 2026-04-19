package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestNewConnectClient(t *testing.T) {
	if _, err := NewConnectClient(ConnectOptions{}); err == nil {
		t.Fatalf("expected error")
	}
	_, err := NewConnectClient(ConnectOptions{Source: cookieSourceStub{cookies: []*http.Cookie{{Name: "sp_dc", Value: "token"}}}})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
}

func TestConnectInfoOperations(t *testing.T) {
	payloads := map[string]map[string]any{
		"getTrack": {
			"data": map[string]any{"track": map[string]any{"uri": "spotify:track:t1", "name": "Song"}},
		},
		"getAlbum": {
			"data": map[string]any{"album": map[string]any{"uri": "spotify:album:a1", "name": "Album"}},
		},
		"queryArtistOverview": {
			"data": map[string]any{"artist": map[string]any{"uri": "spotify:artist:ar1", "name": "Artist"}},
		},
		"fetchPlaylist": {
			"data": map[string]any{"playlist": map[string]any{"uri": "spotify:playlist:p1", "name": "Playlist"}},
		},
		"queryPodcastEpisodes": {
			"data": map[string]any{"show": map[string]any{"uri": "spotify:show:s1", "name": "Show"}},
		},
		"getEpisodeOrChapter": {
			"data": map[string]any{"episode": map[string]any{"uri": "spotify:episode:e1", "name": "Episode"}},
		},
	}
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		op := body.OperationName
		payload, ok := payloads[op]
		if !ok {
			return textResponse(http.StatusNotFound, "missing"), nil
		}
		return jsonResponse(http.StatusOK, payload), nil
	})
	client := newConnectClientForTests(transport)
	for op := range payloads {
		client.hashes.hashes[op] = "hash"
	}
	if item, err := client.GetTrack(context.Background(), "t1"); err != nil || item.ID != "t1" {
		t.Fatalf("track: %#v err=%v", item, err)
	}
	if item, err := client.GetAlbum(context.Background(), "a1"); err != nil || item.ID != "a1" {
		t.Fatalf("album: %#v err=%v", item, err)
	}
	if item, err := client.GetArtist(context.Background(), "ar1"); err != nil || item.ID != "ar1" {
		t.Fatalf("artist: %#v err=%v", item, err)
	}
	if item, err := client.GetPlaylist(context.Background(), "p1"); err != nil || item.ID != "p1" {
		t.Fatalf("playlist: %#v err=%v", item, err)
	}
	if item, err := client.GetShow(context.Background(), "s1"); err != nil || item.ID != "s1" {
		t.Fatalf("show: %#v err=%v", item, err)
	}
	if item, err := client.GetEpisode(context.Background(), "e1"); err != nil || item.ID != "e1" {
		t.Fatalf("episode: %#v err=%v", item, err)
	}
}

func TestConnectUnsupported(t *testing.T) {
	client := &ConnectClient{}
	if _, _, err := client.LibraryTracks(context.Background(), 1, 0); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := client.LibraryAlbums(context.Background(), 1, 0); err == nil {
		t.Fatalf("expected error")
	}
	if err := client.LibraryModify(context.Background(), "/me/tracks", []string{"t1"}, http.MethodPut); err == nil {
		t.Fatalf("expected error")
	}
	if err := client.FollowArtists(context.Background(), []string{"a1"}, http.MethodPut); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, _, err := client.FollowedArtists(context.Background(), 1, ""); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := client.Playlists(context.Background(), 1, 0); err == nil {
		t.Fatalf("expected error")
	}
	if _, _, err := client.PlaylistTracks(context.Background(), "p1", 1, 0); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := client.CreatePlaylist(context.Background(), "name", false, false); err == nil {
		t.Fatalf("expected error")
	}
	// Empty modifications are treated as no-ops.
	if err := client.AddTracks(context.Background(), "p1", nil, nil); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
	if err := client.RemoveTracks(context.Background(), "p1", nil); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
}

func TestConnectAddTracksAppend(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.OperationName != "addToPlaylist" {
			return textResponse(http.StatusNotFound, "missing"), nil
		}
		position, _ := body.Variables["newPosition"].(map[string]any)
		if position["moveType"] != "BOTTOM_OF_PLAYLIST" {
			t.Fatalf("unexpected position: %#v", position)
		}
		return jsonResponse(http.StatusOK, map[string]any{"data": map[string]any{}}), nil
	})
	client := newConnectClientForTests(transport)
	client.hashes.hashes["addToPlaylist"] = "hash"
	if err := client.AddTracks(
		context.Background(),
		"p1",
		[]string{"spotify:track:t1"},
		nil,
	); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
}

func TestConnectAddTracksBeforeUID(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		switch body.OperationName {
		case "fetchPlaylistContents":
			payload := map[string]any{
				"data": map[string]any{
					"playlistV2": map[string]any{
						"content": map[string]any{
							"items": []any{
								map[string]any{"uid": "uid-1"},
							},
						},
					},
				},
			}
			return jsonResponse(http.StatusOK, payload), nil
		case "addToPlaylist":
			position, _ := body.Variables["newPosition"].(map[string]any)
			if position["moveType"] != "BEFORE_UID" {
				t.Fatalf("unexpected moveType: %#v", position)
			}
			if position["fromUid"] != "uid-1" {
				t.Fatalf("unexpected target uid: %#v", position)
			}
			return jsonResponse(http.StatusOK, map[string]any{"data": map[string]any{}}), nil
		default:
			return textResponse(http.StatusNotFound, "missing"), nil
		}
	})
	client := newConnectClientForTests(transport)
	client.hashes.hashes["fetchPlaylistContents"] = "hash"
	client.hashes.hashes["addToPlaylist"] = "hash"
	position := 0
	if err := client.AddTracks(
		context.Background(),
		"p1",
		[]string{"spotify:track:t1"},
		&position,
	); err != nil {
		t.Fatalf("add tracks: %v", err)
	}
}

func TestConnectAddTracksPositionOutOfRange(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.OperationName != "fetchPlaylistContents" {
			return textResponse(http.StatusNotFound, "missing"), nil
		}
		payload := map[string]any{
			"data": map[string]any{
				"playlistV2": map[string]any{
					"content": map[string]any{
						"totalCount": 1,
						"items":      []any{},
					},
				},
			},
		}
		return jsonResponse(http.StatusOK, payload), nil
	})
	client := newConnectClientForTests(transport)
	client.hashes.hashes["fetchPlaylistContents"] = "hash"
	position := 2
	err := client.AddTracks(
		context.Background(),
		"p1",
		[]string{"spotify:track:t1"},
		&position,
	)
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "position 2 is out of range for playlist with 1 tracks" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectUpdatePlaylistUsesChangesEndpoint(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/playlist/v2/playlist/p1/changes":
			if req.Method != http.MethodPost {
				return textResponse(http.StatusMethodNotAllowed, "bad method"), nil
			}
			var body map[string]any
			_ = json.NewDecoder(req.Body).Decode(&body)
			deltas, _ := body["deltas"].([]any)
			ops, _ := deltas[0].(map[string]any)["ops"].([]any)
			op := ops[0].(map[string]any)
			if op["kind"] != "UPDATE_LIST_ATTRIBUTES" {
				t.Fatalf("unexpected op kind: %#v", op)
			}
			newAttrs := op["updateListAttributes"].(map[string]any)["newAttributes"].(map[string]any)
			values := newAttrs["values"].(map[string]any)
			if values["name"] != "Renamed" || values["description"] != "desc" || values["collaborative"] != false {
				t.Fatalf("unexpected values: %#v", values)
			}
			return jsonResponse(http.StatusOK, map[string]any{"revision": "rev-2"}), nil
		case "/playlist/v2/playlist/p1":
			if req.Method != http.MethodGet {
				return textResponse(http.StatusMethodNotAllowed, "bad method"), nil
			}
			if req.URL.Query().Get("decorate") != "revision,length,attributes,timestamp,owner,capabilities" {
				t.Fatalf("unexpected query: %s", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"length":        3,
				"ownerUsername": "me",
				"attributes": map[string]any{
					"name":          "Renamed",
					"description":   "desc",
					"collaborative": false,
				},
			}), nil
		default:
			return textResponse(http.StatusNotFound, req.URL.Path), nil
		}
	})
	client := newConnectClientForTests(transport)
	name := "Renamed"
	description := "desc"
	collaborative := false
	item, err := client.UpdatePlaylist(context.Background(), "p1", PlaylistUpdate{
		Name:          &name,
		Description:   &description,
		Collaborative: &collaborative,
	})
	if err != nil {
		t.Fatalf("update playlist: %v", err)
	}
	if item.Name != name || item.Description != description || item.TotalTracks != 3 {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.Collaborative == nil || *item.Collaborative {
		t.Fatalf("unexpected collaborative flag: %#v", item.Collaborative)
	}
}

func TestConnectCreateCollaborativePlaylistUsesChangesEndpoint(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/playlist/v2/playlist":
			if req.Method != http.MethodPost {
				return textResponse(http.StatusMethodNotAllowed, "bad method"), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"uri": "spotify:playlist:p2"}), nil
		case "/playlist/v2/playlist/p2/changes":
			var body map[string]any
			_ = json.NewDecoder(req.Body).Decode(&body)
			deltas, _ := body["deltas"].([]any)
			ops, _ := deltas[0].(map[string]any)["ops"].([]any)
			newAttrs := ops[0].(map[string]any)["updateListAttributes"].(map[string]any)["newAttributes"].(map[string]any)
			values := newAttrs["values"].(map[string]any)
			if values["collaborative"] != true {
				t.Fatalf("unexpected values: %#v", values)
			}
			return jsonResponse(http.StatusOK, map[string]any{"revision": "rev-3"}), nil
		case "/playlist/v2/playlist/p2":
			return jsonResponse(http.StatusOK, map[string]any{
				"length":        0,
				"ownerUsername": "me",
				"attributes": map[string]any{
					"name":          "Shared",
					"collaborative": true,
				},
			}), nil
		default:
			return textResponse(http.StatusNotFound, req.URL.Path), nil
		}
	})
	client := newConnectClientForTests(transport)
	item, err := client.CreatePlaylist(context.Background(), "Shared", false, true)
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if item.ID != "p2" || item.Collaborative == nil || !*item.Collaborative {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestConnectCreatePlaylistErrorsOnMissingURI(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/playlist/v2/playlist" {
			t.Fatalf("unexpected follow-up request: %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})
	client := newConnectClientForTests(transport)
	if _, err := client.CreatePlaylist(context.Background(), "Shared", false, false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestConnectRecentlyPlayedNormalizesDateToRFC3339(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.OperationName != "recents" {
			return textResponse(http.StatusNotFound, "missing"), nil
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"data": map[string]any{
				"lists": []any{
					map[string]any{
						"items": map[string]any{
							"items": []any{
								map[string]any{
									"entity": map[string]any{
										"uri":  "spotify:track:t1",
										"name": "Song",
									},
									"addedAt": map[string]any{
										"year":  2026,
										"month": 4,
										"day":   18,
									},
								},
							},
						},
					},
				},
			},
		}), nil
	})
	client := newConnectClientForTests(transport)
	client.hashes.hashes["recents"] = "hash"
	items, err := client.RecentlyPlayed(context.Background(), 1)
	if err != nil {
		t.Fatalf("recently played: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected items: %#v", items)
	}
	if _, err := time.Parse(time.RFC3339, items[0].PlayedAt); err != nil {
		t.Fatalf("expected RFC3339 timestamp, got %q: %v", items[0].PlayedAt, err)
	}
	if items[0].PlayedAt != "2026-04-18T00:00:00Z" {
		t.Fatalf("unexpected timestamp: %q", items[0].PlayedAt)
	}
}
