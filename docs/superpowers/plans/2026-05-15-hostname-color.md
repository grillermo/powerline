# Hostname-Seeded Background Color Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `--hostname` is passed, derive a deterministic 256-color background from the short hostname that guarantees white-text contrast.

**Architecture:** FNV-32a hashes the short hostname to a hue angle; HSL(hue, 100%, 20%) converts to RGB; each channel maps to the nearest 256-color cube level; final index = 16 + 36r + 6g + b. L=20% ensures all hues round to cube level 95 or below, keeping contrast ≥ 6.4:1 against white.

**Tech Stack:** Go stdlib — `hash/fnv`, `math`, `os`, `strings` (all already available).

---

### Task 1: Add `hostnameColor()` and helpers to `powerline-zsh.go`

**Files:**
- Modify: `powerline-zsh.go` (add import, add three functions, update one call site)

> Note: project has no test suite (see CLAUDE.md). Verification is build + visual inspection.

- [ ] **Step 1: Add `hash/fnv` and `math` to imports**

Open `powerline-zsh.go`. The current import block is:
```go
import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)
```

Replace with:
```go
import (
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)
```

- [ ] **Step 2: Add `nearestCubeLevel` helper after the color constants block**

After the closing `)` of the `const` block, add:

```go
func nearestCubeLevel(v float64) int {
	levels := [6]float64{0, 95.0 / 255, 135.0 / 255, 175.0 / 255, 215.0 / 255, 1.0}
	best := 0
	bestDist := math.Abs(v - levels[0])
	for i := 1; i < 6; i++ {
		if d := math.Abs(v - levels[i]); d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}
```

- [ ] **Step 3: Add `hslToRGB` helper immediately after `nearestCubeLevel`**

```go
func hslToRGB(h, s, l float64) (float64, float64, float64) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}
```

- [ ] **Step 4: Add `hostnameColor` after `hslToRGB`**

```go
func hostnameColor() int {
	host, _ := os.Hostname()
	if idx := strings.Index(host, "."); idx != -1 {
		host = host[:idx]
	}
	h := fnv.New32a()
	h.Write([]byte(host))
	hue := float64(h.Sum32() % 360)
	r, g, b := hslToRGB(hue, 1.0, 0.20)
	return 16 + 36*nearestCubeLevel(r) + 6*nearestCubeLevel(g) + nearestCubeLevel(b)
}
```

- [ ] **Step 5: Update hostname segment call site in `addCwdSegment`**

Find this block (around line 120):
```go
	if hostname {
		p.append(p.newSegment(" %m ", CWD_FG, PATH_BG, &thinSep, &sepFg))
	}
```

Replace with:
```go
	if hostname {
		hostBg := hostnameColor()
		p.append(p.newSegment(" %m ", 15, hostBg, &thinSep, &sepFg))
	}
```

(`15` = xterm white, maximum contrast against any dark background.)

- [ ] **Step 6: Build and verify it compiles**

```bash
cd /Users/grillermo/c/powerline && go build -o powerline-zsh powerline-zsh.go
```

Expected: no output, exit 0, `powerline-zsh` binary updated.

- [ ] **Step 7: Spot-check determinism and color range**

```bash
./powerline-zsh --hostname 0
```

Run twice — output must be identical. Inspect the `%K{N}` value in the hostname segment; N must be in range 16–231.

- [ ] **Step 8: Commit**

```bash
git add powerline-zsh.go docs/superpowers/specs/2026-05-15-hostname-color-design.md docs/superpowers/plans/2026-05-15-hostname-color.md
git commit -m "feat: derive hostname segment bg color from hostname hash"
```
