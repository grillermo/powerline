# Hostname-Seeded Background Color

**Date:** 2026-05-15  
**Status:** Approved

## Goal

When `--hostname` flag is passed, the hostname segment background uses a deterministic color derived from the short hostname. Same host always gets same color. Color provides good white-text contrast on any host.

## Algorithm

1. `os.Hostname()` → short hostname string
2. FNV-32a hash → `uint32`
3. `hue = hash % 360`
4. HSL(hue, 100%, 20%) → RGB

**Why 20% lightness:** The 256-color cube has coarse channel values: {0, 95, 135, 175, 215, 255}. At L=30%, yellow-range hues (max channel ~0.6) round up to cube level 135, yielding relative luminance ~0.23 and contrast ratio ~3.75:1 against white — below WCAG AA (4.5:1). At L=20%, max channel is 0.4 which rounds down to cube level 95, keeping contrast ≥ 6.4:1 for all hues.

5. Each RGB channel → nearest of {0, 95, 135, 175, 215, 255} → index 0–5
6. `color256 = 16 + 36*r_idx + 6*g_idx + b_idx`

## Code Changes

**File:** `powerline-zsh.go`

- Add import: `hash/fnv`
- Add function `hostnameColor() int` implementing algorithm above
- In `addCwdSegment`: replace `PATH_BG` with `hostnameColor()` for hostname segment bg; change fg from `CWD_FG` (254) to `15` (white) for maximum contrast

## Scope

~30 lines added. No existing logic changes. No new dependencies beyond stdlib.
