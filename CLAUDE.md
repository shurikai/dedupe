# CLAUDE.md — dedupe

## Read This First

Before writing any code or proposing any changes, read the existing source files. Understand the current command structure, package layout, module name, and conventions already in use. Do not invent structure — extend what's there.

---

## Project Overview

`dedupe` is a personal CLI tool for photo deduplication and organization. It is written in Go and uses the Charm/Bubble Tea library suite for interactive TUI components, as well as the Cobra library for CLI management. The project is local-first and operates on the local filesystem.

**GitHub:** https://github.com/shurikai/dedupe (confirm actual repo path from go.mod)

---

## Engineering Philosophy

- **Clarity over cleverness.** Readable code is preferred over terse or "clever" code.
- **Simplicity over unnecessary complexity.** Don't reach for abstractions until they're earned.
- **Prefer stdlib.** Avoid adding third-party dependencies unless they provide significant, clear value. Charm/Bubble Tea and Cobra are already dependencies and are fine to use for TUI/CLI work.
- **No Java/Spring patterns.** This is Go — use Go idioms, not OOP inheritance patterns or over-engineered interfaces.
- **Terminal-first.** The primary user is a developer working in a terminal. UX should reflect that.

---

## Code Style

- Follow standard Go formatting (`gofmt`).
- Error handling: always handle errors explicitly; do not silently discard them.
- No `panic` in normal control flow.
- Keep functions focused and small.
- Prefer flat package structure over deep nesting unless complexity demands it.
- No em-dashes in comments or documentation (personal preference).

---

## Planned New Features

Two new subcommands are being added. Read the existing command structure before deciding where and how to add them.

### 1. `review` subcommand

A local web server that enables visual review of duplicate pairs.

**Input:** A plain text file where each line contains two file paths separated by whitespace. Each pair represents two files (images or video) previously identified as duplicates.

Example pair file:
```
/photos/2023/img_001.jpg  /photos/backup/img_001.jpg
/photos/2023/vid_042.mp4  /photos/backup/vid_042.mp4
```

**Behavior:**
- Starts a local HTTP server (default port configurable, suggest `--port` flag)
- Opens the browser automatically, or prints the URL if it cannot
- Serves the files in the pair list as static assets via a controlled endpoint (see Security note below)
- Exposes a small JSON API:
  - `GET /api/pairs` — returns the list of pairs with index, left path, right path, and current decision
  - `POST /api/decision` — records a decision for a pair by index
  - `GET /api/file?path=...` — serves a file by path (validated against loaded pair list)
- Shuts down cleanly when the user closes the browser tab or sends an interrupt

**Frontend (single HTML file, served from embedded Go assets):**
- Side-by-side display of the two files in each pair
- Detects file type by extension: render `<img>` for images (jpg, jpeg, png, gif, webp, heic), `<video>` for video (mp4, mov, avi, mkv)
- Keyboard shortcuts:
  - `d` — mark as duplicate (left is canonical, delete right)
  - `D` — mark as duplicate (right is canonical, delete left)
  - `k` — keep both (not duplicates)
  - `u` — unsure / skip
  - `→` or `n` — next pair
  - `←` or `p` — previous pair
- Progress indicator (e.g. "12 / 48 reviewed")
- Visual state per pair: unreviewed / decided / unsure
- Decisions persist in memory and are written to the sidecar file on every decision (not just at the end)

**Security:**
- The file serving endpoint MUST validate that the requested path exists in the loaded pair list.
- Do not expose an arbitrary filesystem file server.

### 2. `clean` subcommand

Reads a decisions sidecar file and executes the recorded actions.

**Input:** The JSON decisions file produced by `review`.

**Actions per decision:**
- `delete_left` — delete the left file
- `delete_right` — delete the right file
- `keep_both` — no action
- `unsure` — no action (or optionally list to stdout for manual handling)

**Flags:**
- `--dry-run` — print actions without executing them (default behavior for safety; require explicit `--execute` to actually delete)
- `--execute` — actually perform deletions
- `--move-to <dir>` — instead of deleting, move the marked file to a staging directory

**Safety:**
- `--dry-run` should be the default. Deleting files is destructive and irreversible.
- Print a summary before executing: "X files will be deleted. Y pairs kept. Z unsure."
- Require confirmation prompt before executing unless `--yes` flag is passed.

---

## Decisions File Format

The `review` subcommand writes decisions to a sidecar JSON file alongside the input pair file (e.g. `pairs.txt` → `pairs.decisions.json`). Path is also configurable via `--output` flag.

```json
[
  {
    "left": "/photos/2023/img_001.jpg",
    "right": "/photos/backup/img_001.jpg",
    "decision": "delete_right"
  },
  {
    "left": "/photos/2023/vid_042.mp4",
    "right": "/photos/backup/vid_042.mp4",
    "decision": "unsure"
  }
]
```

Valid decision values: `delete_left`, `delete_right`, `keep_both`, `unsure`, `""` (empty = not yet reviewed).

---

## What Not To Do

- Do not restructure existing code to fit the new features. Add to what's there.
- Do not add an ORM, database, or external storage for the decisions file. JSON on disk is sufficient.
- Do not add a framework for the web server. `net/http` from stdlib is fine.
- Do not generate a full SPA with a build step. The frontend should be a single embedded HTML file with vanilla JS.
- Do not use `os.Exit` deep in library code. Return errors to the caller.
