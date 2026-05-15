use regex::Regex;
use std::env;
use std::io::Write;
use std::path::Path;
use std::process::{Command, Stdio};

// Color constants
const PATH_BG: u32 = 237;
const PATH_FG: u32 = 250;
const CWD_FG: u32 = 254;
const SEPARATOR_FG: u32 = 244;

const REPO_CLEAN_BG: u32 = 148;
const REPO_CLEAN_FG: u32 = 0;
const REPO_DIRTY_BG: u32 = 161;
const REPO_DIRTY_FG: u32 = 15;

const CMD_PASSED_BG: u32 = 236;
const CMD_PASSED_FG: u32 = 15;
const CMD_FAILED_BG: u32 = 161;
const CMD_FAILED_FG: u32 = 15;

const SVN_CHANGES_BG: u32 = 148;
const SVN_CHANGES_FG: u32 = 22;

const VIRTUAL_ENV_BG: u32 = 35;
const VIRTUAL_ENV_FG: u32 = 22;

struct SymbolSet {
    separator: &'static str,
    separator_thin: &'static str,
}

fn symbols_ssh(mode: &str) -> SymbolSet {
    match mode {
        "none" => SymbolSet { separator: "", separator_thin: "" },
        _ => SymbolSet { separator: ">", separator_thin: "|" },
    }
}

fn symbols_normal(mode: &str) -> SymbolSet {
    match mode {
        "none"       => SymbolSet { separator: "", separator_thin: "" },
        "compatible" => SymbolSet { separator: "\u{25B6}", separator_thin: "\u{276F}" },
        "patched"    => SymbolSet { separator: "\u{2B80}", separator_thin: "\u{2B81}" },
        "konsole"    => SymbolSet { separator: "\u{e0b0}", separator_thin: "\u{e0b1}" },
        _            => SymbolSet { separator: "⮀", separator_thin: "⮁" },
    }
}

struct Segment {
    content: String,
    fg: u32,
    bg: u32,
    separator: String,
    separator_fg: u32,
}

struct Powerline {
    separator: String,
    separator_thin: String,
    segments: Vec<Segment>,
}

impl Powerline {
    fn new(mode: &str) -> Self {
        let is_ssh = env::var("SSH_CLIENT").is_ok() || env::var("SSH_TTY").is_ok();
        let sym = if is_ssh { symbols_ssh(mode) } else { symbols_normal(mode) };
        Powerline {
            separator: sym.separator.to_string(),
            separator_thin: sym.separator_thin.to_string(),
            segments: Vec::new(),
        }
    }

    fn fgcolor(&self, code: u32) -> String {
        format!("%F{{{}}}", code)
    }

    fn bgcolor(&self, code: u32) -> String {
        format!("%K{{{}}}", code)
    }

    fn new_segment(
        &self,
        content: String,
        fg: u32,
        bg: u32,
        separator: Option<&str>,
        separator_fg: Option<u32>,
    ) -> Segment {
        Segment {
            content,
            fg,
            bg,
            separator: separator.unwrap_or(&self.separator).to_string(),
            separator_fg: separator_fg.unwrap_or(bg),
        }
    }

    fn append(&mut self, seg: Segment) {
        self.segments.push(seg);
    }

    fn draw(&self) -> String {
        let reset = " %f%k";
        let mut out = String::new();
        let len = self.segments.len();
        for (i, seg) in self.segments.iter().enumerate() {
            let separator_bg = if i + 1 < len {
                self.bgcolor(self.segments[i + 1].bg)
            } else {
                reset.to_string()
            };
            out.push_str(&self.fgcolor(seg.fg));
            out.push_str(&self.bgcolor(seg.bg));
            out.push_str(&seg.content);
            out.push_str(&separator_bg);
            out.push_str(&self.fgcolor(seg.separator_fg));
            out.push_str(&seg.separator);
        }
        out.push_str(reset);
        out
    }
}

fn add_cwd_segment(p: &mut Powerline, maxdepth: usize, cwd_only: bool, hostname: bool) {
    let home = env::var("HOME").unwrap_or_default();
    let cwd = env::var("PWD").unwrap_or_else(|_| {
        env::current_dir()
            .map(|d| d.to_string_lossy().to_string())
            .unwrap_or_default()
    });

    let cwd = if cwd.starts_with(&home) {
        format!("~{}", &cwd[home.len()..])
    } else {
        cwd
    };

    let cwd = if cwd.starts_with('/') { cwd[1..].to_string() } else { cwd };

    let mut names: Vec<String> = cwd.split('/').map(|s| s.to_string()).collect();
    if names.len() > maxdepth {
        let tail_start = names.len() + 2 - maxdepth;
        let mut truncated = names[..2].to_vec();
        truncated.push("⋯ ".to_string());
        truncated.extend_from_slice(&names[tail_start..]);
        names = truncated;
    }

    let thin_sep = p.separator_thin.clone();

    if hostname {
        let seg = p.new_segment(" %m ".to_string(), CWD_FG, PATH_BG, Some(&thin_sep), Some(SEPARATOR_FG));
        p.append(seg);
    }

    let n = names.len();
    if !cwd_only {
        let prefixes: Vec<String> = names[..n - 1].to_vec();
        for name in prefixes {
            let seg = p.new_segment(format!(" {} ", name), PATH_FG, PATH_BG, Some(&thin_sep), Some(SEPARATOR_FG));
            p.append(seg);
        }
    }
    let last = names[n - 1].clone();
    let seg = p.new_segment(format!(" {} ", last), CWD_FG, PATH_BG, None, None);
    p.append(seg);
}

fn get_hg_status() -> (bool, bool, bool) {
    let out = Command::new("hg").arg("status").output();
    let (mut has_modified, mut has_untracked, mut has_missing) = (false, false, false);
    if let Ok(output) = out {
        for line in String::from_utf8_lossy(&output.stdout).split('\n') {
            if line.is_empty() { continue; }
            match line.chars().next() {
                Some('?') => has_untracked = true,
                Some('!') => has_missing = true,
                Some(_)   => has_modified = true,
                None => {}
            }
        }
    }
    (has_modified, has_untracked, has_missing)
}

fn add_hg_segment(p: &mut Powerline) -> bool {
    let out = Command::new("hg").arg("branch").output().ok();
    let out = match out {
        Some(o) if !o.stdout.is_empty() => o,
        _ => return false,
    };
    let branch = String::from_utf8_lossy(&out.stdout).trim_end_matches('\n').to_string();
    if branch.is_empty() { return false; }

    let (has_modified, has_untracked, has_missing) = get_hg_status();

    let (bg, fg, branch) = if has_modified || has_untracked || has_missing {
        let mut extra = String::new();
        if has_untracked { extra.push('+'); }
        if has_missing   { extra.push('!'); }
        let b = if extra.is_empty() { branch } else { format!("{} {}", branch, extra) };
        (REPO_DIRTY_BG, REPO_DIRTY_FG, b)
    } else {
        (REPO_CLEAN_BG, REPO_CLEAN_FG, branch)
    };

    let seg = p.new_segment(format!(" {} ", branch), fg, bg, None, None);
    p.append(seg);
    true
}

fn get_git_status() -> (bool, bool, String) {
    let mut has_pending = true;
    let mut has_untracked = false;
    let mut origin_position = String::new();

    let out = Command::new("git").args(["status", "-unormal"]).output();
    let re = Regex::new(r"Your branch is (ahead|behind).*?(\d+) comm").unwrap();

    if let Ok(output) = out {
        for line in String::from_utf8_lossy(&output.stdout).split('\n') {
            if let Some(caps) = re.captures(line) {
                let n: u32 = caps[2].parse().unwrap_or(0);
                origin_position = format!(" {}", n);
                if &caps[1] == "behind" {
                    origin_position.push('⇣');
                } else {
                    origin_position.push('⇡');
                }
            }
            if line.contains("nothing to commit") { has_pending = false; }
            if line.contains("Untracked files") { has_untracked = true; }
        }
    }
    (has_pending, has_untracked, origin_position)
}

fn add_git_segment(p: &mut Powerline) -> bool {
    let result = Command::new("git")
        .args(["symbolic-ref", "-q", "HEAD"])
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .output();

    let branch = match result {
        Err(_) => return false,
        Ok(out) => {
            if !out.status.success() {
                let stderr = String::from_utf8_lossy(&out.stderr).to_lowercase();
                if stderr.contains("not a git repo") {
                    return false;
                }
                // Detached HEAD
                "(Detached)".to_string()
            } else {
                let ref_str = String::from_utf8_lossy(&out.stdout)
                    .trim_end_matches('\n')
                    .to_string();
                ref_str.strip_prefix("refs/heads/").unwrap_or(&ref_str).to_string()
            }
        }
    };

    let (has_pending, has_untracked, origin_position) = get_git_status();
    let mut branch = format!("{}{}", branch, origin_position);
    if has_untracked { branch.push_str(" +"); }

    let (bg, fg) = if has_pending {
        (REPO_DIRTY_BG, REPO_DIRTY_FG)
    } else {
        (REPO_CLEAN_BG, REPO_CLEAN_FG)
    };

    let seg = p.new_segment(format!(" {} ", branch), fg, bg, None, None);
    p.append(seg);
    true
}

fn add_svn_segment(p: &mut Powerline, cwd: &str) -> bool {
    if !Path::new(&format!("{}/.svn", cwd)).exists() {
        return false;
    }

    let svn = Command::new("svn").arg("status").stdout(Stdio::piped()).spawn();
    let svn = match svn {
        Ok(c) => c,
        Err(_) => return false,
    };

    let grep = Command::new("grep")
        .args(["-c", r"^[ACDIMRX\!\~]"])
        .stdin(svn.stdout.unwrap())
        .output();

    let output = match grep {
        Ok(o) => o,
        Err(_) => return false,
    };

    let changes = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let n: u32 = changes.parse().unwrap_or(0);
    if n == 0 { return false; }

    let seg = p.new_segment(format!(" {} ", changes), SVN_CHANGES_FG, SVN_CHANGES_BG, None, None);
    p.append(seg);
    true
}

fn add_repo_segment(p: &mut Powerline, cwd: &str) {
    if add_git_segment(p) { return; }
    if add_svn_segment(p, cwd) { return; }
    add_hg_segment(p);
}

fn add_virtual_env_segment(p: &mut Powerline) -> bool {
    let env_path = match env::var("VIRTUAL_ENV") {
        Ok(v) => v,
        Err(_) => return false,
    };
    let env_name = Path::new(&env_path)
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
        .unwrap_or(env_path);
    let seg = p.new_segment(format!(" {} ", env_name), VIRTUAL_ENV_FG, VIRTUAL_ENV_BG, None, None);
    p.append(seg);
    true
}

fn add_root_indicator(p: &mut Powerline, prev_error: i32) {
    let (bg, fg) = if prev_error != 0 {
        (CMD_FAILED_BG, CMD_FAILED_FG)
    } else {
        (CMD_PASSED_BG, CMD_PASSED_FG)
    };
    let seg = p.new_segment(" ❄".to_string(), fg, bg, None, None);
    p.append(seg);
}

fn get_valid_cwd() -> String {
    match env::current_dir() {
        Ok(path) => path.to_string_lossy().to_string(),
        Err(_) => {
            let cwd = env::var("PWD").unwrap_or_default();
            let sep = std::path::MAIN_SEPARATOR.to_string();
            let parts: Vec<&str> = cwd.split(sep.as_str()).collect();
            let mut len = parts.len();
            let mut up = cwd.clone();
            loop {
                if Path::new(&up).exists() { break; }
                if len == 0 { break; }
                len -= 1;
                up = parts[..len].join(&sep);
            }
            if env::set_current_dir(&up).is_err() {
                eprintln!("[powerline-zsh] Your current directory is invalid.");
                std::process::exit(1);
            }
            eprintln!("[powerline-zsh] Your current directory is invalid. Lowest valid directory: {}", up);
            cwd
        }
    }
}

fn main() {
    let args: Vec<String> = env::args().collect();

    let mut cwd_only = false;
    let mut hostname = false;
    let mut mode = "default".to_string();
    let mut prev_error: i32 = 0;
    let mut positional: Vec<String> = Vec::new();

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--cwd-only" => cwd_only = true,
            "--hostname" => hostname = true,
            "-m" => {
                i += 1;
                if i < args.len() {
                    mode = args[i].clone();
                }
            }
            arg => {
                if !arg.starts_with('-') {
                    positional.push(arg.to_string());
                }
            }
        }
        i += 1;
    }

    if let Some(first) = positional.first() {
        prev_error = first.parse().unwrap_or(0);
    }

    let mut p = Powerline::new(&mode);
    let cwd = get_valid_cwd();

    add_virtual_env_segment(&mut p);
    add_cwd_segment(&mut p, 5, cwd_only, hostname);
    add_repo_segment(&mut p, &cwd);
    add_root_indicator(&mut p, prev_error);

    let output = p.draw();
    std::io::stdout().write_all(output.as_bytes()).unwrap();
}
