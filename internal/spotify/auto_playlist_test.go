package spotify_test

import (
	"context"
	"testing"

	"github.com/steipete/spogo/internal/spotify"
	"github.com/steipete/spogo/internal/testutil"
)

func TestAutoClientCreatePlaylistUsesConnectClientForCollaborativePlaylists(t *testing.T) {
	auto := spotify.NewAutoClient(
		&testutil.SpotifyMock{
			CreatePlaylistFn: func(
				ctx context.Context,
				name string,
				public bool,
				collaborative bool,
			) (spotify.Item, error) {
				if name != "Shared" || public || !collaborative {
					t.Fatalf("unexpected args: %q %v %v", name, public, collaborative)
				}
				return spotify.Item{ID: "p1", Name: name, Type: "playlist"}, nil
			},
		},
		&testutil.SpotifyMock{
			CreatePlaylistFn: func(context.Context, string, bool, bool) (spotify.Item, error) {
				t.Fatalf("expected connect client for collaborative playlist creation")
				return spotify.Item{}, nil
			},
		},
	)

	item, err := auto.CreatePlaylist(context.Background(), "Shared", false, true)
	if err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if item.ID != "p1" {
		t.Fatalf("unexpected item id: %s", item.ID)
	}
}

func TestAutoClientUpdatePlaylistPrefersConnectAndFallsBackOnUnsupported(t *testing.T) {
	name := "Renamed"
	auto := spotify.NewAutoClient(
		&testutil.SpotifyMock{
			UpdatePlaylistFn: func(context.Context, string, spotify.PlaylistUpdate) (spotify.Item, error) {
				return spotify.Item{}, spotify.ErrUnsupported
			},
		},
		&testutil.SpotifyMock{
			UpdatePlaylistFn: func(_ context.Context, playlistID string, update spotify.PlaylistUpdate) (spotify.Item, error) {
				if playlistID != "p1" {
					t.Fatalf("unexpected playlist id: %s", playlistID)
				}
				if update.Name == nil || *update.Name != name {
					t.Fatalf("unexpected update: %#v", update)
				}
				return spotify.Item{ID: playlistID, Name: name, Type: "playlist"}, nil
			},
		},
	)

	item, err := auto.UpdatePlaylist(context.Background(), "p1", spotify.PlaylistUpdate{Name: &name})
	if err != nil {
		t.Fatalf("update playlist: %v", err)
	}
	if item.Name != name {
		t.Fatalf("unexpected item: %#v", item)
	}
}
