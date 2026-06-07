# AGENTS.md

Project context for AI coding assistants working on this repository.

## Overview

Toofan is a terminal typing test written in Go. It uses Bubble Tea for the TUI and Lipgloss for styling. Everything runs offline — words, code snippets, themes, and user data are all local.

## Build & Run

```sh
go build -o toofan .   # compile binary
go run .               # run without building
go vet ./...           # lint check before committing
```

There are no external services, databases, or network calls. User data lives in `~/.config/toofan/` as local JSON/JSONL files.

## Project Structure

```
main.go                         Entry point. Initializes Bubble Tea program.
internal/
  game/
    game.go                     Core game logic: typing, backspace, stats, timer.
    storage.go                  All file I/O: config, results, PBs, backups.
  tui/
    model.go                    Bubble Tea model, Update loop, View dispatch.
    typing.go                   Typing screen: input handling, live WPM, progress bar.
    results.go                  Post-test results screen.
    profile.go                  Profile dashboard: overview, PBs, ranks, history, heatmap.
    text.go                     Text rendering: syntax coloring, word wrapping, cursor.
    picker.go                   Picker overlays: language, lesson, theme, duration.
  lang/
    lang.go                     Loads embedded word lists and code snippets from data/.
    data/<language>/            One directory per language.
      words.txt                 Word list (one word per line, english only).
      lessons/<topic>.go        Code lessons (hand-written, not generated).
  theme/
    theme.go                    Palette struct, All slice, Next/ByName helpers.
    <name>.go                   One file per theme. Each exports a single Palette var.
```

## Architecture Rules

- `game/` has zero UI dependencies. It must never import `tui/`, `theme/`, or `lipgloss`.
- `tui/` reads game state through accessors (`g.Text()`, `g.Input()`, `g.Errors()`). It never touches game internals directly.
- All file I/O is centralized in `storage.go`. No file reads/writes anywhere else.
- `lang/` uses `embed.FS` to bake data files into the binary at compile time. No runtime file system access for content.

## Adding Content

### New Theme

Create `internal/theme/<name>.go`:

```go
package theme

import "github.com/charmbracelet/lipgloss"

var MyTheme = Palette{
    Name:       "mytheme",
    Background: lipgloss.Color("#1a1a1a"),
    Foreground: lipgloss.Color("#555555"),
    Typed:      lipgloss.Color("#ffffff"),
    Error:      lipgloss.Color("#ff0000"),
    Cursor:     lipgloss.Color("#ffffff"),
    Accent:     lipgloss.Color("#00ff00"),
    Success:    lipgloss.Color("#00ff00"),
}
```

Then add `MyTheme` to the `All` slice in `theme.go`.

### New Language

1. Create `internal/lang/data/<language>/` directory.
2. Add lesson files inside it. Each file is a self-contained code snippet.
3. Every lesson file must start with a `// Topic: <title>` comment line. The comment prefix depends on the language (`//`, `#`, or `--`).
4. The code below the comments is what the user types.
5. Rebuild. The `embed.FS` picks up new files automatically.

### New Word List

Place a `words.txt` inside a language's data directory. One word per line.

## Coding Conventions

- No unnecessary comments. Code should be self-explanatory.
- No emoji in source code or CLI output.
- Error handling: return early, don't wrap in else blocks.
- Naming: short, lowercase Go conventions. No stuttering (`game.GameState` → `game.State`).
- Keep functions short. If a view function exceeds ~60 lines, extract helpers.
- Lipgloss styles are created inline where used. No global style variables.

## Data Format

Config and PBs are stored together in `~/.config/toofan/config.json`:

```
{
  "duration": 30,
  "mode": "words",
  "lang": "go",
  "difficulty": "easy",
  "theme": "tokyonight",
  "pb": {
    "words-30": 105,
    "code-15": 54
  }
}
```

Results are stored in `~/.config/toofan/results.jsonl` (JSON Lines, one test per line):

```
{"at":"2026-04-01 22:18","wpm":85,"acc":97.5,"dur":30,"mode":"words","raw":83,"err":2}
{"at":"2026-04-01 22:20","wpm":54,"acc":91.0,"dur":15,"mode":"code:go","raw":60,"err":5}
```

Races are stored in `~/.config/toofan/races.jsonl` (JSON Lines, capped at the latest 10 records).

On first run after this format change, old `config.txt`/`pb.txt`/`results.txt`/`races.txt` files are auto-migrated to the new format and removed after successful conversion.

## Do Not

- Do not add external API calls or network requests.
- Do not add dependencies beyond Bubble Tea and Lipgloss.
- Do not use AI-generated code snippets for lessons. All lessons are hand-written.
- Do not modify JSON/JSONL data formats without updating both `storage.go` and `profile.go` parsers/readers.
- Do not add global mutable state outside of `theme.Current`.
