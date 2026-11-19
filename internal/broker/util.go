package broker

import (
	"os"
	"path/filepath"
	"sort"

	"filesentry/internal/wire"
)

// describe builds a list entry for name inside dir.
func describe(dir, name string) (wire.Entry, error) {
	full := filepath.Join(dir, name)
	fi, err := os.Lstat(full)
	if err != nil {
		return wire.Entry{}, err
	}

	entry := entryFromInfo(name, fi)
	if uid, gid, ok := ownerOf(fi); ok {
		entry.UID = uid
		entry.GID = gid
	}
	return entry, nil
}

func entryFromInfo(name string, fi os.FileInfo) wire.Entry {
	return wire.Entry{
		Name:  name,
		IsDir: fi.IsDir(),
		Size:  fi.Size(),
		Mode:  uint32(fi.Mode().Perm()),
	}
}

func sortEntries(entries []wire.Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
}
