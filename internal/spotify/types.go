package spotify

type Item struct {
	ID            string   `json:"id"`
	URI           string   `json:"uri"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	URL           string   `json:"url"`
	Artists       []string `json:"artists,omitempty"`
	Album         string   `json:"album,omitempty"`
	Owner         string   `json:"owner,omitempty"`
	DurationMS    int      `json:"duration_ms,omitempty"`
	Explicit      bool     `json:"explicit,omitempty"`
	TotalTracks   int      `json:"total_tracks,omitempty"`
	ReleaseDate   string   `json:"release_date,omitempty"`
	Description   string   `json:"description,omitempty"`
	Picture       string   `json:"picture,omitempty"`
	Public        *bool    `json:"public,omitempty"`
	Collaborative *bool    `json:"collaborative,omitempty"`
	TotalItems    int      `json:"total_items,omitempty"`
	Followers     int      `json:"followers,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	IsPlayable    bool     `json:"is_playable,omitempty"`
	Publisher     string   `json:"publisher,omitempty"`
	TotalEpisodes int      `json:"total_episodes,omitempty"`
	Tracks        []Item   `json:"tracks,omitempty"`
}

type PlaylistUpdate struct {
	Name          *string
	Public        *bool
	Collaborative *bool
	Description   *string
	ImageData     []byte
	ClearImage    bool
}

type SearchResult struct {
	Type   string `json:"type"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Total  int    `json:"total"`
	Items  []Item `json:"items"`
}

type PlaybackStatus struct {
	IsPlaying  bool   `json:"is_playing"`
	ProgressMS int    `json:"progress_ms"`
	Item       *Item  `json:"item,omitempty"`
	Device     Device `json:"device"`
	Shuffle    bool   `json:"shuffle"`
	Repeat     string `json:"repeat"`
}

type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Volume     int    `json:"volume_percent"`
	Active     bool   `json:"is_active"`
	Restricted bool   `json:"is_restricted"`
}

type Queue struct {
	CurrentlyPlaying *Item  `json:"currently_playing,omitempty"`
	Queue            []Item `json:"queue"`
}

type RecentItem struct {
	Track    Item   `json:"track"`
	PlayedAt string `json:"played_at"`
}
