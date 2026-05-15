# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

```bash
cargo build --release
# Binary: target/release/powerline-zsh
```

No tests exist. No linting configured.

## Architecture

Powerline-style zsh prompt generator. Three implementations:
- `powerline-zsh.go` — Go implementation, the main one
- `powerline-zsh.py` — original Python implementation (for reference only, ignore it)

### How it works

`Powerline` struct holds a list of `Segment`s. Each segment has content, 256-color fg/bg codes, and a separator. `draw()` converts segments into zsh prompt escape sequences (`%F{n}`, `%K{n}`).

Segment providers append to `Powerline` in this order:
1. `add_virtual_env_segment` — reads `$VIRTUAL_ENV`
2. `add_cwd_segment` — reads `$HOME`/`$PWD`, truncates deep paths with `⋯`
3. `add_repo_segment` — tries git → svn → hg in order, first match wins
4. `add_root_indicator` — shows previous command exit code (red on failure)

### CLI interface

```
powerline-zsh [prev_exit_code] [--cwd-only] [--hostname] [-m <mode>]
```

- `prev_exit_code` — positional arg, drives `add_root_indicator` color
- `-m` — symbol mode: `default`, `none`, `compatible`, `patched`, `konsole`
- SSH sessions auto-use ASCII separators instead of Unicode powerline glyphs

### Color constants

All colors are 256-color terminal indices defined at the top of `src/main.rs`. Clean/dirty repo states use different bg colors (148 green vs 161 red).
