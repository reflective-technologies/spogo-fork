package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/steipete/spogo/internal/app"
)

type HistoryCmd struct {
	Limit int `help:"Limit results." default:"20"`
}

func (cmd *HistoryCmd) Run(ctx *app.Context) error {
	client, err := ctx.Spotify()
	if err != nil {
		return err
	}
	limit := cmd.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	items, err := client.RecentlyPlayed(context.Background(), limit)
	if err != nil {
		return err
	}

	var plain, human []string
	jsonItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		playedAt := item.PlayedAt
		if t, err := time.Parse(time.RFC3339, playedAt); err == nil {
			playedAt = t.Local().Format("Jan 02 15:04")
		}

		artists := strings.Join(item.Track.Artists, ", ")
		plain = append(plain, fmt.Sprintf("%s\t%s\t%s", item.Track.ID, artists, item.Track.Name))
		human = append(human, fmt.Sprintf("%s  %s — %s", playedAt, artists, item.Track.Name))
		jsonItems = append(jsonItems, map[string]any{
			"played_at": item.PlayedAt,
			"track":     item.Track,
		})
	}

	payload := map[string]any{"items": jsonItems}
	return ctx.Output.Emit(payload, plain, human)
}
