package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/steipete/spogo/internal/output"
	"github.com/steipete/spogo/internal/spotify"
	"github.com/steipete/spogo/internal/testutil"
)

func TestPlaylistAddCmd(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	called := false
	mock := &testutil.SpotifyMock{
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			called = true
			if playlistID != "p1" {
				t.Fatalf("playlist id %s", playlistID)
			}
			if len(uris) != 1 || uris[0] != "spotify:track:t1" {
				t.Fatalf("uris: %#v", uris)
			}
			if position != nil {
				t.Fatalf("expected append, got position %d", *position)
			}
			return nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistAddCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:track:t1"},
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected call")
	}
}

func TestPlaylistAddCmdError(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			return errors.New("boom")
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistAddCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:track:t1"},
	}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistPrependCmdAlbum(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	called := false
	mock := &testutil.SpotifyMock{
		GetTrackFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{}, errors.New("not a track")
		},
		GetAlbumFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{
				ID:   id,
				Type: "album",
				Name: "Album",
				Tracks: []spotify.Item{
					{ID: "t1", URI: "spotify:track:t1", Type: "track"},
					{ID: "t2", URI: "spotify:track:t2", Type: "track"},
				},
			}, nil
		},
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			called = true
			if playlistID != "p1" {
				t.Fatalf("playlist id %s", playlistID)
			}
			if len(uris) != 2 ||
				uris[0] != "spotify:track:t1" ||
				uris[1] != "spotify:track:t2" {
				t.Fatalf("uris: %#v", uris)
			}
			if position == nil || *position != 0 {
				t.Fatalf("expected prepend position 0, got %#v", position)
			}
			return nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistPrependCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:album:a1"},
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected call")
	}
}

func TestPlaylistPrependCmdBareAlbumID(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		GetTrackFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{}, errors.New("not a track")
		},
		GetAlbumFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{
				ID:   id,
				Type: "album",
				Name: "Album",
				URI:  "spotify:album:" + id,
				Tracks: []spotify.Item{
					{ID: "t1", URI: "spotify:track:t1", Type: "track"},
					{ID: "t2", URI: "spotify:track:t2", Type: "track"},
				},
			}, nil
		},
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			if len(uris) != 2 {
				t.Fatalf("uris: %#v", uris)
			}
			if position == nil || *position != 0 {
				t.Fatalf("position: %#v", position)
			}
			return nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistPrependCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"a1"},
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestPlaylistAddCmdSkipsInvalidAlbumTracks(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		GetAlbumFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{
				ID:   id,
				Type: "album",
				Name: "Album",
				URI:  "spotify:album:" + id,
				Tracks: []spotify.Item{
					{ID: "bad", URI: "", Type: "track"},
					{ID: "local", URI: "spotify:local:artist:album:song", Type: "track"},
					{ID: "ok", URI: "spotify:track:t1", Type: "track"},
				},
			}, nil
		},
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			if len(uris) != 1 || uris[0] != "spotify:track:t1" {
				t.Fatalf("uris: %#v", uris)
			}
			return nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistAddCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:album:a1"},
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestResolveUntypedPlaylistItemReturnsCombinedError(t *testing.T) {
	mock := &testutil.SpotifyMock{
		GetTrackFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{ID: id, Type: "track"}, nil
		},
		GetAlbumFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{}, errors.New("album lookup failed")
		},
	}
	_, _, err := resolveUntypedPlaylistItem(mock, "x1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "track x1 has no URI") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "album lookup failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlaylistCreateCmd(t *testing.T) {
	ctx, out, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		CreatePlaylistFn: func(ctx context.Context, name string, public, collaborative bool) (spotify.Item, error) {
			return spotify.Item{ID: "p1", Name: name, Type: "playlist"}, nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistCreateCmd{Name: "Road Trip"}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.String() == "" {
		t.Fatalf("expected output")
	}
}

func TestPlaylistTracksCmd(t *testing.T) {
	ctx, out, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		PlaylistTracksFn: func(ctx context.Context, id string, limit, offset int) ([]spotify.Item, int, error) {
			return []spotify.Item{{ID: "t1", Name: "Track", Type: "track"}}, 1, nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistTracksCmd{Playlist: "spotify:playlist:p1", Limit: 1}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.String() == "" {
		t.Fatalf("expected output")
	}
}

func TestPlaylistRemoveCmd(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	mock := &testutil.SpotifyMock{
		RemoveTracksFn: func(ctx context.Context, playlistID string, uris []string) error {
			if playlistID != "p1" {
				t.Fatalf("playlist %s", playlistID)
			}
			return nil
		},
	}
	ctx.SetSpotify(mock)
	cmd := PlaylistRemoveCmd{Playlist: "spotify:playlist:p1", Tracks: []string{"spotify:track:t1"}}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}
