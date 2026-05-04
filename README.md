# Plex to Jellyfin

CLI and web UI tool to migrate Plex watch states (watched, unwatched, partially watched) to Jellyfin for a target Plex and Jellyfin user. Not vibe coded.

![Last release](https://img.shields.io/github/release/qdm12/plex-to-jellyfin?label=Last%20release)
![Last Docker tag](https://img.shields.io/docker/v/qmcgaw/plex-to-jellyfin?sort=semver&label=Last%20Docker%20tag)
![GitHub last release date](https://img.shields.io/github/release-date/qdm12/plex-to-jellyfin?label=Last%20release%20date)
![Commits since release](https://img.shields.io/github/commits-since/qdm12/plex-to-jellyfin/latest?sort=semver)

[![GitHub last commit](https://img.shields.io/github/last-commit/qdm12/plex-to-jellyfin.svg)](https://github.com/qdm12/plex-to-jellyfin/commits/main)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/y/qdm12/plex-to-jellyfin.svg)](https://github.com/qdm12/plex-to-jellyfin/graphs/contributors)
[![GitHub closed PRs](https://img.shields.io/github/issues-pr-closed/qdm12/plex-to-jellyfin.svg)](https://github.com/qdm12/plex-to-jellyfin/pulls?q=is%3Apr+is%3Aclosed)
[![GitHub issues](https://img.shields.io/github/issues/qdm12/plex-to-jellyfin.svg)](https://github.com/qdm12/plex-to-jellyfin/issues)
[![GitHub closed issues](https://img.shields.io/github/issues-closed/qdm12/plex-to-jellyfin.svg)](https://github.com/qdm12/plex-to-jellyfin/issues?q=is%3Aissue+is%3Aclosed)

![Visitors count](https://visitor-badge.laobi.icu/badge?page_id=plex-to-jellyfin.readme)
[![Build status](https://github.com/qdm12/plex-to-jellyfin/actions/workflows/ci.yml/badge.svg)](https://github.com/qdm12/plex-to-jellyfin/actions/workflows/ci.yml)

## Features

- CLI operation allows to migrate from Plex to Jellyfin
- Web UI operation allows to migrate from Plex to Jellyfin AND for users to fetch their Plex token using their username and password, optionally OTP, and send it to the admin (aka *you*) running the migration
- Migration migrates:
  - watch states from Plex to Jellyfin: watched, unwatched and partially watched (playback position)
- Media are matched using this order:
    1. External provider ID (IMDb/TMDb/TVDb) match
    1. File path match - *useful when running Plex and Jellyfin using the same media directories*
    1. Title approximate match ([code](./internal/migrate/migrator.go#L288)) - *less reliable but it is a nice backup solution*
- Supports dry-run mode for safe verification before writing
- Pre-built standalone binary programs built for Linux, OSX and Windows
- Docker images available on on the [GitHub Container Registry](https://github.com/users/qdm12/packages/container/package/plex-to-jellyfin)
- Docker images are multi-arch: `linux/amd64`, `linux/386`, `linux/arm64`, `linux/arm/v6`, `linux/arm/v7`.

## Usage

You can run the program either as a standalone binary or as a Docker container:

- Standalone binary
    1. Download the binary matching your platform from [the releases page](https://github.com/qdm12/plex-to-jellyfin/releases).
  You can also install the binary from source using [Go](https://golang.org/dl/) with `go install github.com/qdm12/plex-to-jellyfin/cmd`
    1. On Linux and OSX, make the binary executable with `chmod +x plex-to-jellyfin`
    1. Run the binary:

        ```sh
        # Web UI - migration or Plex token retrieval
        ./plex-to-jellyfin
        # CLI migration
        ./plex-to-jellyfin migrate --plex-url=x --plex-token=x --jellyfin-url=x --jellyfin-user=x --jellyfin-api-key=x
        ```

- Docker container

    ```sh
    # Web UI - migration or Plex token retrieval
    docker run --rm -p 8080:8080/tcp ghcr.io/qdm12/plex-to-jellyfin
    # CLI migration
    docker run --rm -e PLEX_URL=x -e PLEX_TOKEN=x -e JELLYFIN_URL=x -e JELLYFIN_USER=x -e JELLYFIN_API_KEY=x ghcr.io/qdm12/plex-to-jellyfin migrate
    ```

  There also Docker tags matching each Github release, for example `:0.1.0` for release `v0.1.0`.
  The latest image tag matches the tip of the main branch.

### Options

| Flag | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `--plex-url` | `PLEX_URL` | | Plex server base URL (example: `http://plex:32400`) |
| `--plex-token` | `PLEX_TOKEN` | | Plex user API token |
| `--plex-libraries` | `PLEX_LIBRARIES` | | Comma-separated Plex library names to include; empty means all movie/show libraries |
| `--plex-send-token-to` | `PLEX_SEND_TOKEN_TO` | | Optional text to display on the web ui to send the user Plex token to |
| `--jellyfin-url` | `JELLYFIN_URL` | | Jellyfin server base URL (example: `http://jellyfin:8096`) |
| `--jellyfin-user` | `JELLYFIN_USER` | | Jellyfin username to update watch data for |
| `--jellyfin-api-key` | `JELLYFIN_API_KEY` | | Jellyfin API key |
| `--dry-run` | `DRY_RUN` | `false` | Print migration actions without writing to Jellyfin |
| `--webui-listening-address` | `WEBUI_LISTENING_ADDRESS` | `:8080` | Web UI listening address |
| `--log-level` | `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. `debug` logs all matches. |

## Getting tokens

### Plex token

Run the program without a subcommand to start the web UI, then click on "Obtain Plex token" and follow the instructions.

### Jellyfin API key

1. Sign in to Jellyfin with an admin account.
1. Open Dashboard.
1. Go to API Keys.
1. Create a new API key.
1. Copy the generated key and use it as `JELLYFIN_API_KEY`.
