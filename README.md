# AoC Discord Bot

A Discord bot for a private [Advent of Code](https://adventofcode.com) leaderboard.

Forked from [PaytonWebber/aoc-discord-bot](https://github.com/PaytonWebber/aoc-discord-bot) (MIT).

## Features

- **Puzzle announcements**: posts a message (optionally pinging a role) when each day's puzzle unlocks, at midnight US Eastern time (05:00 UTC).
- **Star notifications**: one message per completed part, e.g. `🌟 Alice completed Day 3 – Part 2 at 06:12 on 3 Dec`, in your time zone.
- **Live leaderboard**: a single pinned message that is edited after every update, instead of new messages filling the channel.
- **New member notifications** when someone joins the private leaderboard.
- Commands: `!leaderboard`, `!stars`, `!update`, `!help`.

The bot polls the leaderboard at most once every **15 minutes**, as Advent of Code asks, including across restarts. Star messages show the real solve time from the API, so a message can arrive up to 15 minutes after the solve.

## Setup

### 1. Discord application

1. Create an application and a bot in the [Developer Portal](https://discord.com/developers/applications).
2. Under **Bot**, enable **MESSAGE CONTENT INTENT**. Without it the bot refuses to start.

   ![image](images/bot_message_content.png)

3. Invite the bot to your server with these permissions in the announcement channel: **View Channel**, **Send Messages**, **Embed Links**, **Read Message History** and **Pin Messages**.
4. Enable Developer Mode in Discord (User Settings → Advanced), then right-click the channel → *Copy Channel ID*. For role pings, right-click the role → *Copy Role ID*.

### 2. Advent of Code

- **Leaderboard ID**: the number in your private leaderboard's URL, `https://adventofcode.com/<year>/leaderboard/private/view/<ID>`. The account whose cookie you use must be a member.
- **Session cookie**: log in to adventofcode.com, open the browser dev tools → Storage/Application → Cookies, and copy the value of `session`.
  - Treat it like a password.
  - It expires after a while. If the bot logs `unexpected status ... is SESSION_COOKIE valid`, copy a fresh one.

### 3. Configuration

Copy `.env.example` to `.env` and fill it in.

| Variable | Required | Default | Description |
|---|---|---|---|
| `SESSION_COOKIE` | yes | | AoC `session` cookie |
| `LEADERBOARD_ID` | yes | | Private leaderboard ID |
| `DISCORD_TOKEN` | yes | | Bot token |
| `CHANNEL_ID` | yes | | Channel the bot posts in and listens to |
| `CONTACT` | recommended | | Your contact (e.g. GitHub profile URL), sent in the User-Agent as AoC asks |
| `AOC_YEAR` | no | current year | Event year (2015 or later) |
| `AOC_DAYS` | no | `12` | Number of puzzles (12 since 2025, 25 before) |
| `PING_ROLE_ID` | no | | Role mentioned in puzzle announcements |
| `TIMEZONE` | no | `Europe/Lisbon` | IANA time zone used for times in messages |
| `DATA_DIR` | no | `.` (`/data` in Docker) | Where `leaderboard.json` and `state.json` are stored |

## Running

### Docker (recommended)

```sh
docker compose up -d --build
docker compose logs -f
```

Data is kept in the `bot-data` volume, so restarts never re-announce stars or puzzles.

### Go

Requires Go 1.26 or newer.

```sh
make
./bot
```

### Hosting

The bot needs to run continuously for the whole event. Options:

| Option | Pros | Cons |
|---|---|---|
| Oracle Cloud Always Free VM | Free and always on | Account setup takes a while |
| Raspberry Pi / home server | Free, under your control | Down if your machine or internet is |
| Small paid VPS | Simple and reliable | A few euros per month |

## Usage

![image](images/help_command.png)

![image](images/leaderboard_command.png)

![image](images/stars_command.png)

## Development

```sh
go test -race ./...
go vet ./...
```
