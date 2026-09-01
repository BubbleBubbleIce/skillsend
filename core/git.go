package core

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CacheRootOverride lets tests point the upstream clone cache at a temp dir.
var CacheRootOverride string

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg != "" {
			return out.String(), fmtErr("git %s: %s", strings.Join(args, " "), msg)
		}
		return out.String(), fmtErr("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(dir string) bool {
	_, err := runGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// StatusPorcelain returns hub-relative changed paths ("XY path" lines).
func StatusPorcelain(dir string) ([]string, error) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// porcelainPaths extracts just the path column from porcelain output.
// Renames/copies are reported as "old -> new"; only the new path is kept.
func porcelainPaths(lines []string) []string {
	var paths []string
	for _, l := range lines {
		if len(l) <= 3 {
			continue
		}
		p := l[3:]
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		paths = append(paths, unquoteGitPath(p))
	}
	return paths
}

// unquoteGitPath decodes a path as emitted by `git status --porcelain` with
// core.quotePath on: double-quoted and C-escaped. Returns the input unchanged
// when it isn't quoted.
func unquoteGitPath(p string) string {
	if len(p) < 2 || p[0] != '"' {
		return p
	}
	p = p[1 : len(p)-1]
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c != '\\' || i+1 >= len(p) {
			b.WriteByte(c)
			continue
		}
		i++
		switch p[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			// octal escape \ooo (up to 3 digits)
			if p[i] >= '0' && p[i] <= '7' {
				val, n := 0, 0
				for i < len(p) && n < 3 && p[i] >= '0' && p[i] <= '7' {
					val = val*8 + int(p[i]-'0')
					i++
					n++
				}
				i--
				b.WriteByte(byte(val))
			} else {
				b.WriteByte('\\')
				b.WriteByte(p[i])
			}
		}
	}
	return b.String()
}

// PullFF fast-forwards the hub repo; refuses to merge.
func PullFF(dir string) error {
	_, err := runGit(dir, "pull", "--ff-only")
	return err
}

// Push pushes the current branch to origin.
func Push(dir string) error {
	_, err := runGit(dir, "push")
	return err
}

// CommitAll stages everything and commits; returns false when there was nothing to commit.
func CommitAll(dir, message string) (bool, error) {
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := runGit(dir, "diff", "--cached", "--quiet"); err == nil {
		return false, nil
	}
	_, err := runGit(dir, "commit", "-m", message)
	if err != nil {
		return false, err
	}
	return true, nil
}

// HeadSha returns the current HEAD commit sha of a repo.
func HeadSha(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// HasLocalChanges reports whether a skill directory's current content signature
// differs from the synced snapshot recorded in the manifest. Covers both
// uncommitted and committed local changes.
func HasLocalChanges(hub, rel string, meta SkillMeta) (bool, error) {
	if meta.Tree == "" {
		return true, nil // no baseline recorded — treat as unknown/diverged
	}
	sig, err := DirSignature(filepath.Join(hub, filepath.FromSlash(rel)))
	if err != nil {
		return false, err
	}
	return sig != meta.Tree, nil
}

func cacheRoot() (string, error) {
	if CacheRootOverride != "" {
		return CacheRootOverride, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "skillsend", "upstreams"), nil
}

// EnsureBareClone returns a bare clone cache of the upstream URL, refreshed.
func EnsureBareClone(url string) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(url))
	dir := filepath.Join(root, hex.EncodeToString(sum[:])+".git")
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return "", err
		}
		if _, err := runGit("", "clone", "--bare", url, dir); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if _, err := runGit(dir, "fetch", "origin", "+refs/heads/*:refs/heads/*", "--prune"); err != nil {
		return "", err
	}
	return dir, nil
}

// resolveUpstreamHead resolves the head sha of an upstream ref inside a bare cache.
func resolveUpstreamHead(cacheDir, ref string) (string, error) {
	spec := "HEAD"
	if ref != "" {
		spec = "refs/heads/" + ref
	}
	out, err := runGit(cacheDir, "rev-parse", spec)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// BehindCount counts upstream commits since the synced snapshot; -1 when unknown
// (e.g. the cached clone predates the synced sha).
func BehindCount(cacheDir, synced, head string) int {
	if synced == "" || synced == head {
		return 0
	}
	out, err := runGit(cacheDir, "rev-list", "--count", synced+".."+head)
	if err != nil {
		return -1
	}
	var n int
	if _, err := fmt.Sscan(strings.TrimSpace(out), &n); err != nil {
		return -1
	}
	return n
}

// UpdateUpstreamSkill fast-forwards a skill directory to its upstream head.
// It refuses when the local content diverged from the synced snapshot. Returns
// the updated meta (new Synced sha + new Tree signature) for the caller to
// persist in the manifest. The resulting changes stay uncommitted in the hub
// for the user to review and commit.
//
// Safety: the upstream tree is fully materialized into a staging directory
// before the current one is replaced (rename swap with rollback), so a failure
// mid-extract never leaves the skill half-written.
func UpdateUpstreamSkill(hub, rel string, meta SkillMeta) (SkillMeta, error) {
	if meta.Tree == "" {
		return meta, ErrNoBaseline
	}
	skillDir := filepath.Join(hub, filepath.FromSlash(rel))
	if meta.Synced == "" {
		// Upstream recorded without a synced sha (e.g. added via `e`): establish
		// the baseline by comparing content signatures. Equal → adopt the
		// upstream head as baseline; different → treat as diverged local work.
		cacheDir, err := EnsureBareClone(meta.Source)
		if err != nil {
			return meta, err
		}
		head, err := resolveUpstreamHead(cacheDir, meta.Ref)
		if err != nil {
			return meta, err
		}
		sig, err := signatureOfUpstreamTree(cacheDir, head, meta.Path)
		if err != nil {
			return meta, err
		}
		local, err := DirSignature(skillDir)
		if err != nil {
			return meta, err
		}
		if local != sig {
			return meta, ErrDiverged
		}
		meta.Synced = head
		return meta, nil
	}

	diverged, err := HasLocalChanges(hub, rel, meta)
	if err != nil {
		return meta, err
	}
	if diverged {
		return meta, ErrDiverged
	}
	cacheDir, err := EnsureBareClone(meta.Source)
	if err != nil {
		return meta, err
	}
	head, err := resolveUpstreamHead(cacheDir, meta.Ref)
	if err != nil {
		return meta, err
	}
	if head == meta.Synced {
		return meta, nil // already up to date
	}

	// Materialize the replacement into staging first — nothing is touched
	// inside the hub until the new tree is complete.
	staging, err := os.MkdirTemp(hub, ".skillsend-stage-")
	if err != nil {
		return meta, err
	}
	stageSkill := filepath.Join(staging, "skill")
	if err := os.MkdirAll(stageSkill, 0o755); err != nil {
		os.RemoveAll(staging)
		return meta, err
	}
	if err := materialize(cacheDir, head, meta.Path, stageSkill); err != nil {
		os.RemoveAll(staging)
		return meta, err
	}
	sig, err := DirSignature(stageSkill)
	if err != nil {
		os.RemoveAll(staging)
		return meta, err
	}
	defer os.RemoveAll(staging)

	// Swap: current → backup, staged → current; roll back on any failure.
	backup, err := os.MkdirTemp(hub, ".skillsend-back-")
	if err != nil {
		return meta, err
	}
	defer os.RemoveAll(backup)
	backupSkill := filepath.Join(backup, "skill")
	if err := os.Rename(skillDir, backupSkill); err != nil {
		return meta, err
	}
	if err := os.Rename(stageSkill, skillDir); err != nil {
		os.Rename(backupSkill, skillDir) // roll back
		return meta, err
	}
	meta.Synced = head
	meta.Tree = sig
	return meta, nil
}

// signatureOfUpstreamTree materializes an upstream tree into a temp dir and
// returns its content signature.
func signatureOfUpstreamTree(cacheDir, head, sub string) (string, error) {
	tmp, err := os.MkdirTemp("", "skillsend-sig-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := materialize(cacheDir, head, sub, tmp); err != nil {
		return "", err
	}
	return DirSignature(tmp)
}

// materialize extracts a commit's tree (optionally limited to a subdirectory,
// stripping its prefix) into dst via git archive | tar.
func materialize(cacheDir, sha, sub, dst string) error {
	tmp, err := os.MkdirTemp("", "skillsend-extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	args := []string{"-C", cacheDir, "archive", "--format=tar", sha}
	if sub != "" {
		args = append(args, sub)
	}
	archive := exec.Command("git", args...)
	tar := exec.Command("tar", "-x", "-C", tmp)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	tar.Stdin = pipe
	var errOut bytes.Buffer
	tar.Stderr = &errOut
	if err := archive.Start(); err != nil {
		return err
	}
	if err := tar.Run(); err != nil {
		return fmtErr("extract upstream tree: %s", strings.TrimSpace(errOut.String()))
	}
	if err := archive.Wait(); err != nil {
		return err
	}

	src := tmp
	if sub != "" {
		src = filepath.Join(tmp, filepath.FromSlash(sub))
		if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
			return fmtErr("upstream has no directory %q at %s", sub, head7(sha))
		}
	}
	return moveContents(src, dst)
}

// moveContents moves every entry of src into dst (cross-device safe).
func moveContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		if err := os.Rename(from, to); err != nil {
			if err := copyTree(from, to); err != nil {
				return err
			}
			if err := os.RemoveAll(from); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.MkdirAll(dst, fi.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !fi.Mode().IsRegular() {
		return nil // skip symlinks/special files during cross-device fallback
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, fi.Mode().Perm())
}

func head7(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Staleness is the update outlook for one upstream-backed skill.
type Staleness struct {
	Name     string
	Behind   int // -1 unknown
	UpToDate bool
	Diverged bool
	Err      error
}

// CheckStaleness fetches every upstream in the manifest and reports how far
// behind each skill is and whether local changes block a fast-forward.
func CheckStaleness(hub string, m *Manifest) []Staleness {
	var out []Staleness
	names := sortedNames(m.Skills)
	for _, name := range names {
		meta := m.Skills[name]
		st := Staleness{Name: name}
		skill, ok := FindSkillRel(hub, name)
		if !ok {
			st.Err = fmtErr("skill %q not found in hub", name)
			out = append(out, st)
			continue
		}
		cacheDir, err := EnsureBareClone(meta.Source)
		if err != nil {
			st.Err = err
			out = append(out, st)
			continue
		}
		head, err := resolveUpstreamHead(cacheDir, meta.Ref)
		if err != nil {
			st.Err = err
			out = append(out, st)
			continue
		}
		st.Behind = BehindCount(cacheDir, meta.Synced, head)
		st.UpToDate = head == meta.Synced
		diverged, err := HasLocalChanges(hub, skill, meta)
		if err != nil {
			st.Err = err
		}
		st.Diverged = diverged
		out = append(out, st)
	}
	return out
}

// ShortSha abbreviates a commit sha for display.
func ShortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// FindSkillRel locates a skill's hub-relative path by leaf name (top or nested one level).
func FindSkillRel(hub, leaf string) (string, bool) {
	direct := filepath.Join(hub, leaf)
	if fi, err := os.Stat(filepath.Join(direct, "SKILL.md")); err == nil && !fi.IsDir() {
		return leaf, true
	}
	entries, err := os.ReadDir(hub)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		nested := filepath.Join(hub, e.Name(), leaf)
		if fi, err := os.Stat(filepath.Join(nested, "SKILL.md")); err == nil && !fi.IsDir() {
			return e.Name() + "/" + leaf, true
		}
	}
	return "", false
}
