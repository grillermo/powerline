package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Color constants
const (
	PATH_BG      = 237
	PATH_FG      = 250
	CWD_FG       = 254
	SEPARATOR_FG = 244

	REPO_CLEAN_BG = 148
	REPO_CLEAN_FG = 0
	REPO_DIRTY_BG = 161
	REPO_DIRTY_FG = 15

	CMD_PASSED_BG = 236
	CMD_PASSED_FG = 15
	CMD_FAILED_BG = 161
	CMD_FAILED_FG = 15

	SVN_CHANGES_BG = 148
	SVN_CHANGES_FG = 22

	VIRTUAL_ENV_BG = 35
	VIRTUAL_ENV_FG = 22
)

func hostnameColor() int {
	// Hand-verified 256-color indices: all have contrast ratio ≥ 4.5:1 against white.
	palette := []int{
		17, 18, 19, 21,      // navy → bright blue
		22, 23, 24, 28,      // dark green → teal
		52, 88, 124,         // maroon → red
		53, 55, 56, 90, 91,  // magenta → violet
		58, 94, 130,         // olive → burnt orange
		161,                 // hot pink
	}
	host, _ := os.Hostname()
	if idx := strings.Index(host, "."); idx != -1 {
		host = host[:idx]
	}
	if len(host) > 5 {
		host = host[:5]
	}
	h := fnv.New32a()
	h.Write([]byte(host))
	return palette[h.Sum32()%uint32(len(palette))]
}

type symbolSet struct {
	separator     string
	separatorThin string
}

var symbolsSSH = map[string]symbolSet{
	"none":       {"", ""},
	"compatible": {">", "|"},
	"patched":    {">", "|"},
	"konsole":    {">", "|"},
	"default":    {">", "|"},
}

var symbolsNormal = map[string]symbolSet{
	"none":       {"", ""},
	"compatible": {"\u25B6", "\u276F"},
	"patched":    {"\u2B80", "\u2B81"},
	"konsole":    {"\ue0b0", "\ue0b1"},
	"default":    {"⮀", "⮁"},
}

type Segment struct {
	content     string
	fg          int
	bg          int
	separator   string
	separatorFg int
}

type Powerline struct {
	separator     string
	separatorThin string
	segments      []Segment
}

func newPowerline(mode string) *Powerline {
	isSSH := os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""

	var sym symbolSet
	if isSSH {
		sym = symbolsSSH[mode]
	} else {
		sym = symbolsNormal[mode]
	}

	return &Powerline{
		separator:     sym.separator,
		separatorThin: sym.separatorThin,
	}
}

func (p *Powerline) fgcolor(code int) string {
	return fmt.Sprintf("%%F{%d}", code)
}

func (p *Powerline) bgcolor(code int) string {
	return fmt.Sprintf("%%K{%d}", code)
}

func (p *Powerline) newSegment(content string, fg, bg int, separator *string, separatorFg *int) Segment {
	sep := p.separator
	if separator != nil {
		sep = *separator
	}
	sepFg := bg
	if separatorFg != nil {
		sepFg = *separatorFg
	}
	return Segment{content: content, fg: fg, bg: bg, separator: sep, separatorFg: sepFg}
}

func (p *Powerline) append(s Segment) {
	p.segments = append(p.segments, s)
}

func (p *Powerline) draw() string {
	reset := " %f%k"
	var sb strings.Builder
	for i, seg := range p.segments {
		var separatorBg string
		if i+1 < len(p.segments) {
			separatorBg = p.bgcolor(p.segments[i+1].bg)
		} else {
			separatorBg = reset
		}
		sb.WriteString(p.fgcolor(seg.fg))
		sb.WriteString(p.bgcolor(seg.bg))
		sb.WriteString(seg.content)
		sb.WriteString(separatorBg)
		sb.WriteString(p.fgcolor(seg.separatorFg))
		sb.WriteString(seg.separator)
	}
	sb.WriteString(reset)
	return sb.String()
}

func addCwdSegment(p *Powerline, maxdepth int, cwdOnly bool, hostname bool) {
	home := os.Getenv("HOME")
	cwd := os.Getenv("PWD")

	if strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}

	if len(cwd) > 0 && cwd[0] == '/' {
		cwd = cwd[1:]
	}

	names := strings.Split(cwd, "/")
	if len(names) > maxdepth {
		names = append(names[:2], append([]string{"⋯ "}, names[2+len(names)-maxdepth:]...)...)
	}

	thinSep := p.separatorThin
	sepFg := SEPARATOR_FG

	if hostname {
		hostBg := hostnameColor()
		p.append(p.newSegment(" %m ", 15, hostBg, &thinSep, &sepFg))
	}

	if !cwdOnly {
		for _, n := range names[:len(names)-1] {
			p.append(p.newSegment(" "+n+" ", PATH_FG, PATH_BG, &thinSep, &sepFg))
		}
	}
	p.append(p.newSegment(" "+names[len(names)-1]+" ", CWD_FG, PATH_BG, nil, nil))
}

func getHgStatus() (hasModified, hasUntracked, hasMissing bool) {
	out, err := exec.Command("hg", "status").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case '?':
			hasUntracked = true
		case '!':
			hasMissing = true
		default:
			hasModified = true
		}
	}
	return
}

func addHgSegment(p *Powerline) bool {
	out, err := exec.Command("hg", "branch").Output()
	if err != nil || len(out) == 0 {
		return false
	}
	branch := strings.TrimRight(string(out), "\n")
	if branch == "" {
		return false
	}

	bg := REPO_CLEAN_BG
	fg := REPO_CLEAN_FG

	hasModified, hasUntracked, hasMissing := getHgStatus()
	if hasModified || hasUntracked || hasMissing {
		bg = REPO_DIRTY_BG
		fg = REPO_DIRTY_FG
		extra := ""
		if hasUntracked {
			extra += "+"
		}
		if hasMissing {
			extra += "!"
		}
		if extra != "" {
			branch += " " + extra
		}
	}
	p.append(p.newSegment(" "+branch+" ", fg, bg, nil, nil))
	return true
}

var originRe = regexp.MustCompile(`Your branch is (ahead|behind).*?(\d+) comm`)

func getGitStatus() (hasPendingCommits, hasUntracked bool, originPosition string) {
	hasPendingCommits = true
	out, err := exec.Command("git", "status", "-unormal").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if m := originRe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			originPosition = fmt.Sprintf(" %d", n)
			if m[1] == "behind" {
				originPosition += "⇣"
			} else {
				originPosition += "⇡"
			}
		}
		if strings.Contains(line, "nothing to commit") {
			hasPendingCommits = false
		}
		if strings.Contains(line, "Untracked files") {
			hasUntracked = true
		}
	}
	return
}

// branchDupesPath reports whether the trailing components of cwd are exactly the
// slash-separated components of branch — the case of a worktree checked out at a
// path named after its own branch, e.g. ~/c/grillermo/sc-35457/dc-fix on branch
// grillermo/sc-35457/dc-fix.
func branchDupesPath(branch, cwd string) bool {
	if branch == "" {
		return false
	}
	parts := strings.Split(branch, "/")
	dirs := strings.Split(filepath.ToSlash(strings.TrimRight(cwd, "/")), "/")
	if len(parts) > len(dirs) {
		return false
	}
	for i, part := range parts {
		if part != dirs[len(dirs)-len(parts)+i] {
			return false
		}
	}
	return true
}

func addGitSegment(p *Powerline, cwd string) bool {
	out, err := exec.Command("git", "symbolic-ref", "-q", "HEAD").Output()
	if err != nil {
		// Check if it's "not a git repo" vs detached HEAD
		errOut := []byte{}
		if exitErr, ok := err.(*exec.ExitError); ok {
			errOut = exitErr.Stderr
		}
		if strings.Contains(strings.ToLower(string(errOut)), "not a git repo") {
			return false
		}
		// Detached HEAD
		out = nil
	}

	var branch string
	if len(out) > 0 {
		ref := strings.TrimRight(string(out), "\n")
		branch = strings.TrimPrefix(ref, "refs/heads/")
	} else {
		branch = "(Detached)"
	}

	if branchDupesPath(branch, cwd) {
		// The path already spells out the branch; keep the segment (its color
		// carries the clean/dirty state) but drop the redundant name.
		branch = "⎇"
	}

	hasPendingCommits, hasUntracked, originPosition := getGitStatus()
	branch += originPosition
	if hasUntracked {
		branch += " +"
	}

	bg := REPO_CLEAN_BG
	fg := REPO_CLEAN_FG
	if hasPendingCommits {
		bg = REPO_DIRTY_BG
		fg = REPO_DIRTY_FG
	}

	p.append(p.newSegment(" "+branch+" ", fg, bg, nil, nil))
	return true
}

func addSvnSegment(p *Powerline, cwd string) bool {
	if _, err := os.Stat(cwd + "/.svn"); os.IsNotExist(err) {
		return false
	}

	svnCmd := exec.Command("svn", "status")
	grepCmd := exec.Command("grep", "-c", `^[ACDIMRX\!\~]`)

	pipe, err := svnCmd.StdoutPipe()
	if err != nil {
		return false
	}
	grepCmd.Stdin = pipe

	if err := svnCmd.Start(); err != nil {
		return false
	}
	out, err := grepCmd.Output()
	svnCmd.Wait()
	if err != nil {
		return false
	}

	changes := strings.TrimSpace(string(out))
	if len(changes) == 0 {
		return false
	}
	n, err := strconv.Atoi(changes)
	if err != nil || n == 0 {
		return false
	}
	p.append(p.newSegment(" "+changes+" ", SVN_CHANGES_FG, SVN_CHANGES_BG, nil, nil))
	return true
}

func addRepoSegment(p *Powerline, cwd string) {
	for _, fn := range []func() bool{
		func() bool { return addGitSegment(p, cwd) },
		func() bool { return addSvnSegment(p, cwd) },
		func() bool { return addHgSegment(p) },
	} {
		if fn() {
			return
		}
	}
}

func addVirtualEnvSegment(p *Powerline) bool {
	env := os.Getenv("VIRTUAL_ENV")
	if env == "" {
		return false
	}
	parts := strings.Split(env, "/")
	envName := parts[len(parts)-1]
	p.append(p.newSegment(" "+envName+" ", VIRTUAL_ENV_FG, VIRTUAL_ENV_BG, nil, nil))
	return true
}

func addRootIndicator(p *Powerline, prevError int) {
	bg := CMD_PASSED_BG
	fg := CMD_PASSED_FG
	if prevError != 0 {
		bg = CMD_FAILED_BG
		fg = CMD_FAILED_FG
	}
	p.append(p.newSegment(" ❄", fg, bg, nil, nil))
}

func getValidCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = os.Getenv("PWD")
		parts := strings.Split(cwd, string(os.PathSeparator))
		up := cwd
		for len(parts) > 0 {
			if _, err := os.Stat(up); err == nil {
				break
			}
			parts = parts[:len(parts)-1]
			up = strings.Join(parts, string(os.PathSeparator))
		}
		if err2 := os.Chdir(up); err2 != nil {
			fmt.Fprintln(os.Stderr, "[powerline-zsh] Your current directory is invalid.")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "[powerline-zsh] Your current directory is invalid. Lowest valid directory: "+up)
	}
	return cwd
}

func main() {
	cwdOnly := flag.Bool("cwd-only", false, "Hide parent directory")
	hostname := flag.Bool("hostname", false, "Show hostname at the begin")
	mode := flag.String("m", "default", "Choose icon font: default, none, compatible, patched or konsole")
	flag.Parse()

	prevError := 0
	if args := flag.Args(); len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			prevError = n
		}
	}

	p := newPowerline(*mode)
	cwd := getValidCwd()

	addVirtualEnvSegment(p)
	addCwdSegment(p, 5, *cwdOnly, *hostname)
	addRepoSegment(p, cwd)
	addRootIndicator(p, prevError)

	os.Stdout.WriteString(p.draw())
}
