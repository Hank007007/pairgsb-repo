package keystore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// KeyVersion is one KEK generation in a key ring.
type KeyVersion struct {
	Version   int        `json:"version"`
	KEK       []byte     `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	RetiredAt *time.Time `json:"retired_at,omitempty"`
}

type keyVersionJSON struct {
	Version   int    `json:"version"`
	KEK       string `json:"kek"`
	CreatedAt string `json:"created_at"`
	RetiredAt string `json:"retired_at,omitempty"`
}

type ringJSON struct {
	GracePeriod string           `json:"grace_period,omitempty"`
	Versions    []keyVersionJSON `json:"versions"`
}

// Ring holds the KEK versions available to a Sealer. It is loaded from a
// JSON file and is immutable afterwards.
type Ring struct {
	Versions    []KeyVersion
	GracePeriod time.Duration
}

// LoadRing reads and validates a key ring file.
func LoadRing(path string) (*Ring, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseRing(data)
}

// ParseRing decodes a key ring from JSON.
func ParseRing(data []byte) (*Ring, error) {
	var raw ringJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRing, err)
	}
	if len(raw.Versions) == 0 {
		return nil, ErrEmptyRing
	}

	ring := &Ring{}
	if raw.GracePeriod != "" {
		d, err := time.ParseDuration(raw.GracePeriod)
		if err != nil || d < 0 {
			return nil, fmt.Errorf("%w: invalid grace_period %q", ErrInvalidRing, raw.GracePeriod)
		}
		ring.GracePeriod = d
	}

	for _, rv := range raw.Versions {
		if rv.Version <= 0 {
			return nil, fmt.Errorf("%w: invalid version %d", ErrInvalidRing, rv.Version)
		}
		kek, err := base64.StdEncoding.DecodeString(rv.KEK)
		if err != nil {
			return nil, fmt.Errorf("%w: version %d: bad kek encoding", ErrInvalidRing, rv.Version)
		}
		if len(kek) != 32 {
			return nil, fmt.Errorf("%w: version %d: kek must be 32 bytes", ErrInvalidRing, rv.Version)
		}
		kv := KeyVersion{Version: rv.Version, KEK: kek}
		if rv.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, rv.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("%w: version %d: bad created_at", ErrInvalidRing, rv.Version)
			}
			kv.CreatedAt = t
		}
		if rv.RetiredAt != "" {
			t, err := time.Parse(time.RFC3339, rv.RetiredAt)
			if err != nil {
				return nil, fmt.Errorf("%w: version %d: bad retired_at", ErrInvalidRing, rv.Version)
			}
			kv.RetiredAt = &t
		}
		replaced := false
		for i := range ring.Versions {
			if ring.Versions[i].Version == kv.Version {
				ring.Versions[i] = kv
				replaced = true
				break
			}
		}
		if !replaced {
			ring.Versions = append(ring.Versions, kv)
		}
	}
	sort.Slice(ring.Versions, func(i, j int) bool {
		return ring.Versions[i].Version < ring.Versions[j].Version
	})
	return ring, nil
}

func (r *Ring) find(version int) (*KeyVersion, bool) {
	for i := range r.Versions {
		if r.Versions[i].Version == version {
			return &r.Versions[i], true
		}
	}
	return nil, false
}

// KEK returns the key material for version.
func (r *Ring) KEK(version int) ([]byte, error) {
	if r == nil {
		return nil, ErrEmptyRing
	}
	kv, ok := r.find(version)
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownVersion, version)
	}
	return kv.KEK, nil
}

// ActiveKEK returns the newest version that has not been retired, together
// with a copy of its key material.
func (r *Ring) ActiveKEK() (int, []byte, error) {
	if r == nil || len(r.Versions) == 0 {
		return 0, nil, ErrEmptyRing
	}
	for i := len(r.Versions) - 1; i >= 0; i-- {
		if r.Versions[i].RetiredAt == nil {
			return r.Versions[i].Version, r.Versions[i].KEK, nil
		}
	}
	return 0, nil, ErrNoActiveVersion
}

// LatestVersion returns the highest version present in the ring.
func (r *Ring) LatestVersion() int {
	if r == nil || len(r.Versions) == 0 {
		return 0
	}
	return r.Versions[len(r.Versions)-1].Version
}

// Clone returns a deep copy of the ring.
func (r *Ring) Clone() *Ring {
	if r == nil {
		return nil
	}
	out := &Ring{GracePeriod: r.GracePeriod, Versions: make([]KeyVersion, len(r.Versions))}
	copy(out.Versions, r.Versions)
	for i := range out.Versions {
		if r.Versions[i].RetiredAt != nil {
			t := *r.Versions[i].RetiredAt
			out.Versions[i].RetiredAt = &t
		}
		out.Versions[i].KEK = append([]byte(nil), r.Versions[i].KEK...)
	}
	return out
}

// Marshal renders the ring back to its JSON file form.
func (r *Ring) Marshal() ([]byte, error) {
	raw := ringJSON{Versions: make([]keyVersionJSON, 0, len(r.Versions))}
	if r.GracePeriod != 0 {
		raw.GracePeriod = r.GracePeriod.String()
	}
	for _, kv := range r.Versions {
		j := keyVersionJSON{
			Version: kv.Version,
			KEK:     base64.StdEncoding.EncodeToString(kv.KEK),
		}
		if !kv.CreatedAt.IsZero() {
			j.CreatedAt = kv.CreatedAt.UTC().Format(time.RFC3339)
		}
		if kv.RetiredAt != nil {
			j.RetiredAt = kv.RetiredAt.UTC().Format(time.RFC3339)
		}
		raw.Versions = append(raw.Versions, j)
	}
	return json.MarshalIndent(raw, "", "  ")
}

// WriteRing atomically writes r to path.
func (r *Ring) WriteRing(path string) error {
	data, err := r.Marshal()
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o600)
}
