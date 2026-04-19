# Changelog

## 0.2.0-kiosk.3 - 2026-04-19

- Add `playlist update` CLI support for renaming and editing playlist metadata in place
- Route playlist metadata writes through the connect playlist changes endpoint
- Support collaborative playlist create/update in the connect engine
- Normalize connect recently-played timestamps to RFC3339
- Error cleanly on malformed or missing connect playlist creation response URIs

## 0.2.0 - 2026-01-07

- Add `applescript` engine for direct Spotify.app control on macOS (thanks @adam91holt)
- CI: bump golangci-lint-action to support golangci-lint v2

## 0.1.0 - 2026-01-02

- Kong-powered CLI with global flags, config profiles, and env overrides
- Auth commands: cookie status/import/clear with browser/profile selection
- Cookie-based auth via steipete/sweetcookie (file cache + browser sources)
- Search tracks/albums/artists/playlists/shows/episodes
- Item info for track/album/artist/playlist/show/episode
- Playback control: play/pause/next/prev/seek/volume/shuffle/repeat/status
- Artist play (top tracks; falls back to search)
- Queue add/show
- Library list/add/remove for tracks/albums; follow/unfollow artists; playlists list
- Playlist management: create/add/remove/track list
- Device list and transfer/set
- Engines: connect (internal), web (Web API), auto (connect → web fallback)
- Rate-limit fallback on 429s where supported
- Output: human color + `--plain` + `--json` (NO_COLOR/TERM aware)
- GitHub Actions CI, linting, formatting, and coverage gate
