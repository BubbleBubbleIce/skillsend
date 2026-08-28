package core

import "errors"

var (
	// ErrDiverged means local content differs from the synced upstream snapshot;
	// a fast-forward would clobber it, so the operation is refused.
	ErrDiverged = errors.New("skill has local changes; refusing to overwrite")
	// ErrNoBaseline means no synced snapshot is recorded for the skill.
	ErrNoBaseline = errors.New("no synced baseline recorded for this skill")
	// ErrConflict means the destination is occupied by something the tool must not touch.
	ErrConflict = errors.New("destination exists and is not a hub link")
	// ErrNotLink means the path is a real directory or file, never a link.
	ErrNotLink = errors.New("path is not a symlink")
	// ErrOutsideHub means a link does not resolve into the hub.
	ErrOutsideHub = errors.New("link does not resolve into the hub")
)
