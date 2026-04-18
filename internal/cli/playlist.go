package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/steipete/spogo/internal/app"
	"github.com/steipete/spogo/internal/output"
	"github.com/steipete/spogo/internal/spotify"
)

type PlaylistCreateCmd struct {
	Name   string `arg:"" required:"" help:"Playlist name."`
	Public bool   `help:"Create public playlist."`
	Collab bool   `help:"Create collaborative playlist."`
}

type PlaylistUpdateCmd struct {
	Playlist    string `arg:"" required:"" help:"Playlist ID/URL/URI."`
	Name        string `help:"New playlist name."`
	Description string `help:"New playlist description."`
	Public      bool   `help:"Set playlist public."`
	Private     bool   `help:"Set playlist private."`
	Collab      bool   `help:"Set playlist collaborative."`
	NonCollab   bool   `name:"non-collab" help:"Set playlist non-collaborative."`
}

type PlaylistAddCmd struct {
	Playlist string   `arg:"" required:"" help:"Playlist ID/URL/URI."`
	Items    []string `arg:"" required:"" help:"Track or album IDs/URLs/URIs."`
	Position *int     `help:"Zero-based insertion position. Defaults to append."`
}

type PlaylistRemoveCmd struct {
	Playlist string   `arg:"" required:"" help:"Playlist ID/URL/URI."`
	Tracks   []string `arg:"" required:"" help:"Track IDs/URLs/URIs."`
}

type PlaylistPrependCmd struct {
	Playlist string   `arg:"" required:"" help:"Playlist ID/URL/URI."`
	Items    []string `arg:"" required:"" help:"Track or album IDs/URLs/URIs."`
}

type PlaylistTracksCmd struct {
	Playlist string `arg:"" required:"" help:"Playlist ID/URL/URI."`
	Limit    int    `help:"Limit results." default:"50"`
	Offset   int    `help:"Offset results." default:"0"`
}

func (cmd *PlaylistCreateCmd) Run(ctx *app.Context) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	item, err := client.CreatePlaylist(context.Background(), cmd.Name, cmd.Public, cmd.Collab)
	if err != nil {
		return err
	}
	plain := []string{itemPlain(item)}
	human := []string{fmt.Sprintf("Created %s", itemHuman(ctx.Output, item))}
	return ctx.Output.Emit(item, plain, human)
}

func (cmd *PlaylistUpdateCmd) Run(ctx *app.Context) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	playlist, err := spotify.ParseTypedID(cmd.Playlist, "playlist")
	if err != nil {
		return err
	}
	update, err := cmd.updatePayload()
	if err != nil {
		return err
	}
	item, err := client.UpdatePlaylist(context.Background(), playlist.ID, update)
	if err != nil {
		return err
	}
	plain := []string{itemPlain(item)}
	human := []string{fmt.Sprintf("Updated %s", itemHuman(ctx.Output, item))}
	return ctx.Output.Emit(item, plain, human)
}

func (cmd *PlaylistAddCmd) Run(ctx *app.Context) error {
	return runPlaylistAdd(ctx, cmd.Playlist, cmd.Items, cmd.Position)
}

func (cmd *PlaylistPrependCmd) Run(ctx *app.Context) error {
	position := 0
	return runPlaylistAdd(ctx, cmd.Playlist, cmd.Items, &position)
}

func runPlaylistAdd(
	ctx *app.Context,
	playlistInput string,
	inputs []string,
	position *int,
) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	playlist, err := spotify.ParseTypedID(playlistInput, "playlist")
	if err != nil {
		return err
	}
	uris, err := playlistItemURIs(client, inputs)
	if err != nil {
		return err
	}
	if err := validatePosition(position); err != nil {
		return err
	}
	if err := client.AddTracks(context.Background(), playlist.ID, uris, position); err != nil {
		return err
	}
	plain := []string{"ok"}
	human := []string{fmt.Sprintf("Added %d tracks", len(uris))}
	payload := map[string]any{"status": "ok", "count": len(uris)}
	if position != nil {
		human = []string{fmt.Sprintf("Added %d tracks at position %d", len(uris), *position)}
		payload["position"] = *position
	}
	return ctx.Output.Emit(payload, plain, human)
}

func (cmd *PlaylistRemoveCmd) Run(ctx *app.Context) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	playlist, err := spotify.ParseTypedID(cmd.Playlist, "playlist")
	if err != nil {
		return err
	}
	uris, err := trackURIs(cmd.Tracks)
	if err != nil {
		return err
	}
	if err := client.RemoveTracks(context.Background(), playlist.ID, uris); err != nil {
		return err
	}
	plain := []string{"ok"}
	human := []string{fmt.Sprintf("Removed %d tracks", len(uris))}
	return ctx.Output.Emit(map[string]any{"status": "ok", "count": len(uris)}, plain, human)
}

func (cmd *PlaylistTracksCmd) Run(ctx *app.Context) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	playlist, err := spotify.ParseTypedID(cmd.Playlist, "playlist")
	if err != nil {
		return err
	}
	limit := clampLimit(cmd.Limit)
	items, total, err := client.PlaylistTracks(context.Background(), playlist.ID, limit, cmd.Offset)
	if err != nil {
		return err
	}
	plain, human := renderItems(ctx.Output, items)
	if ctx.Output.Format == output.FormatHuman {
		human = append([]string{fmt.Sprintf("Tracks: %d", total)}, human...)
	}
	payload := map[string]any{"total": total, "items": items}
	return ctx.Output.Emit(payload, plain, human)
}

func playlistItemURIs(client spotify.API, inputs []string) ([]string, error) {
	uris := make([]string, 0, len(inputs))
	for _, input := range inputs {
		res, err := spotify.ParseResource(strings.TrimSpace(input))
		if err != nil {
			return nil, err
		}
		switch res.Type {
		case "":
			track, album, err := resolveUntypedPlaylistItem(client, res.ID)
			if err != nil {
				return nil, err
			}
			if track != nil {
				uris = append(uris, track.URI)
				continue
			}
			if album != nil {
				albumURIs, err := albumTrackURIs(*album, input)
				if err != nil {
					return nil, err
				}
				uris = append(uris, albumURIs...)
				continue
			}
			return nil, fmt.Errorf("invalid playlist item %s", input)
		case "track":
			if res.URI == "" {
				return nil, fmt.Errorf("invalid track input")
			}
			uris = append(uris, res.URI)
		case "album":
			album, err := client.GetAlbum(context.Background(), res.ID)
			if err != nil {
				return nil, err
			}
			albumURIs, err := albumTrackURIs(album, input)
			if err != nil {
				return nil, err
			}
			uris = append(uris, albumURIs...)
		default:
			return nil, fmt.Errorf("playlist add supports tracks and albums, got %s", res.Type)
		}
	}
	return uris, nil
}

func resolveUntypedPlaylistItem(
	client spotify.API,
	id string,
) (*spotify.Item, *spotify.Item, error) {
	track, trackErr := client.GetTrack(context.Background(), id)
	if trackErr == nil && track.URI != "" {
		return &track, nil, nil
	}
	if trackErr == nil {
		trackErr = fmt.Errorf("track %s has no URI", id)
	}

	album, albumErr := client.GetAlbum(context.Background(), id)
	if albumErr == nil && album.URI != "" {
		return nil, &album, nil
	}
	if albumErr == nil {
		albumErr = fmt.Errorf("album %s has no URI", id)
	}

	return nil, nil, fmt.Errorf(
		"could not resolve Spotify item %s as track (%v) or album (%v)",
		id,
		trackErr,
		albumErr,
	)
}

func albumTrackURIs(album spotify.Item, input string) ([]string, error) {
	if len(album.Tracks) == 0 {
		return nil, fmt.Errorf("album %s has no playable tracks", input)
	}
	uris := make([]string, 0, len(album.Tracks))
	for _, track := range album.Tracks {
		if track.URI == "" || !strings.HasPrefix(track.URI, "spotify:track:") {
			continue
		}
		uris = append(uris, track.URI)
	}
	if len(uris) == 0 {
		return nil, fmt.Errorf("album %s has no playable tracks", input)
	}
	return uris, nil
}

func validatePosition(position *int) error {
	if position != nil && *position < 0 {
		return fmt.Errorf("position must be zero or greater")
	}
	return nil
}

func (cmd *PlaylistUpdateCmd) updatePayload() (spotify.PlaylistUpdate, error) {
	update := spotify.PlaylistUpdate{}
	if strings.TrimSpace(cmd.Name) != "" {
		name := cmd.Name
		update.Name = &name
	}
	if cmd.Description != "" {
		description := cmd.Description
		update.Description = &description
	}
	if cmd.Public && cmd.Private {
		return spotify.PlaylistUpdate{}, fmt.Errorf("cannot set both public and private")
	}
	if cmd.Public {
		value := true
		update.Public = &value
	}
	if cmd.Private {
		value := false
		update.Public = &value
	}
	if cmd.Collab && cmd.NonCollab {
		return spotify.PlaylistUpdate{}, fmt.Errorf("cannot set both collab and non-collab")
	}
	if cmd.Collab {
		value := true
		update.Collaborative = &value
	}
	if cmd.NonCollab {
		value := false
		update.Collaborative = &value
	}
	if update.Name == nil && update.Description == nil && update.Public == nil && update.Collaborative == nil {
		return spotify.PlaylistUpdate{}, fmt.Errorf("no playlist updates requested")
	}
	return update, nil
}

func trackURIs(inputs []string) ([]string, error) {
	uris := make([]string, 0, len(inputs))
	for _, input := range inputs {
		res, err := spotify.ParseTypedID(strings.TrimSpace(input), "track")
		if err != nil {
			return nil, err
		}
		if res.URI == "" {
			return nil, fmt.Errorf("invalid track input")
		}
		uris = append(uris, res.URI)
	}
	return uris, nil
}
