# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

```bash
go build -o powerline-zsh powerline-zsh.go
# Binary: ./powerline-zsh
```

No tests exist. No linting configured.

## Architecture

Powerline-style zsh prompt generator, implemented in `powerline-zsh.go`.

### How it works

`Powerline` struct holds a list of `Segment`s. Each segment has content, 256-color fg/bg codes, and a separator. `draw()` converts segments into zsh prompt escape sequences (`%F{n}`, `%K{n}`).

Segment providers append to `Powerline` in this order:
1. `addVirtualEnvSegment` — reads `$VIRTUAL_ENV`
2. `addCwdSegment` — reads `$HOME`/`$PWD`, truncates deep paths with `⋯`; optionally prepends a hostname segment
3. `addRepoSegment` — tries git → svn → hg in order, first match wins
4. `addRootIndicator` — shows previous command exit code (red on failure)

### CLI interface

```
powerline-zsh [--cwd-only] [--hostname] [-m <mode>] [prev_exit_code]
```

Uses Go's `flag` package, so flags must precede the positional argument.

- `prev_exit_code` — positional arg, drives `addRootIndicator` color
- `-m` — symbol mode: `default`, `none`, `compatible`, `patched`, `konsole`
- SSH sessions auto-use ASCII separators instead of Unicode powerline glyphs

### Color constants

All colors are 256-color terminal indices defined at the top of `powerline-zsh.go`. Clean/dirty repo states use different bg colors (148 green vs 161 red). `hostnameColor()` picks the hostname segment bg from a curated palette by hashing the first 5 chars of the short hostname.
