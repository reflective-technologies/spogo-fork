package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/steipete/spogo/internal/output"
	"github.com/steipete/spogo/internal/spotify"
	"github.com/steipete/spogo/internal/testutil"
)

func TestPlaylistAddCmdInvalidPlaylist(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			return nil
		},
	})
	cmd := PlaylistAddCmd{
		Playlist: "spotify:track:t1",
		Items:    []string{"spotify:track:t1"},
	}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistAddCmdRejectsUnsupportedType(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		AddTracksFn: func(
			ctx context.Context,
			playlistID string,
			uris []string,
			position *int,
		) error {
			return nil
		},
	})
	cmd := PlaylistAddCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:artist:a1"},
	}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistAddCmdAlbumWithoutTracks(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		GetAlbumFn: func(ctx context.Context, id string) (spotify.Item, error) {
			return spotify.Item{ID: id, Type: "album", Name: "Album"}, nil
		},
	})
	cmd := PlaylistAddCmd{
		Playlist: "spotify:playlist:p1",
		Items:    []string{"spotify:album:a1"},
	}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistRemoveCmdError(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		RemoveTracksFn: func(ctx context.Context, playlistID string, uris []string) error {
			return errors.New("boom")
		},
	})
	cmd := PlaylistRemoveCmd{Playlist: "spotify:playlist:p1", Tracks: []string{"spotify:track:t1"}}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistTracksCmdHumanHeader(t *testing.T) {
	ctx, out, _ := testutil.NewTestContext(t, output.FormatHuman)
	ctx.SetSpotify(&testutil.SpotifyMock{
		PlaylistTracksFn: func(ctx context.Context, id string, limit, offset int) ([]spotify.Item, int, error) {
			return []spotify.Item{{ID: "t1", Name: "Track", Type: "track"}}, 1, nil
		},
	})
	cmd := PlaylistTracksCmd{Playlist: "spotify:playlist:p1", Limit: 1}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "Tracks:") {
		t.Fatalf("expected header")
	}
}

func TestPlaylistCreateCmdError(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		CreatePlaylistFn: func(ctx context.Context, name string, public, collaborative bool) (spotify.Item, error) {
			return spotify.Item{}, errors.New("boom")
		},
	})
	cmd := PlaylistCreateCmd{Name: "Fail"}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPlaylistUpdateCmd(t *testing.T) {
	ctx, out, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		UpdatePlaylistFn: func(
			ctx context.Context,
			playlistID string,
			update spotify.PlaylistUpdate,
		) (spotify.Item, error) {
			if playlistID != "p1" {
				t.Fatalf("playlistID = %s", playlistID)
			}
			if update.Name == nil || *update.Name != "Renamed" {
				t.Fatalf("unexpected name: %#v", update.Name)
			}
			if update.Description == nil || *update.Description != "New description" {
				t.Fatalf("unexpected description: %#v", update.Description)
			}
			if update.Public == nil || *update.Public {
				t.Fatalf("unexpected public: %#v", update.Public)
			}
			if update.Collaborative == nil || !*update.Collaborative {
				t.Fatalf("unexpected collaborative: %#v", update.Collaborative)
			}
			return spotify.Item{ID: playlistID, Name: "Renamed", Type: "playlist"}, nil
		},
	})
	cmd := PlaylistUpdateCmd{
		Playlist:    "spotify:playlist:p1",
		Name:        "Renamed",
		Description: func() *string { value := "New description"; return &value }(),
		Private:     true,
		Collab:      true,
	}
	if err := cmd.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.String() == "" {
		t.Fatalf("expected output")
	}
}

func TestPlaylistUpdateCmdParsesEmptyDescriptionAsExplicitUpdate(t *testing.T) {
	command := New()
	parser, err := kong.New(command, kong.Name("spogo"), kong.Vars(VersionVars()))
	if err != nil {
		t.Fatalf("parser: %v", err)
	}
	if _, err := parser.Parse([]string{"playlist", "update", "spotify:playlist:p1", "--description="}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	update, err := command.Playlist.Update.updatePayload()
	if err != nil {
		t.Fatalf("update payload: %v", err)
	}
	if update.Description == nil || *update.Description != "" {
		t.Fatalf("expected explicit empty description update, got %#v", update.Description)
	}
}

func TestPlaylistTracksCmdError(t *testing.T) {
	ctx, _, _ := testutil.NewTestContext(t, output.FormatPlain)
	ctx.SetSpotify(&testutil.SpotifyMock{
		PlaylistTracksFn: func(ctx context.Context, id string, limit, offset int) ([]spotify.Item, int, error) {
			return nil, 0, errors.New("boom")
		},
	})
	cmd := PlaylistTracksCmd{Playlist: "spotify:playlist:p1", Limit: 1}
	if err := cmd.Run(ctx); err == nil {
		t.Fatalf("expected error")
	}
}
