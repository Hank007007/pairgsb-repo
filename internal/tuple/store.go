package tuple

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Revision identifies a point in the store's history. Revision zero denotes
// the empty view; every effective change advances the revision by one.
type Revision uint64

// Errors reported when a requested revision cannot be served.
var (
	// ErrFutureRevision means the requested revision is newer than the
	// store's current revision.
	ErrFutureRevision = errors.New("tuple: revision is newer than the store")

	// ErrStaleRevision means the requested revision has been reclaimed by
	// garbage collection and can no longer be materialised.
	ErrStaleRevision = errors.New("tuple: revision has been reclaimed")
)

type entryKey struct {
	Object   Object
	ID       string
	Relation string
	Subject  Subject
}

func newEntryKey(object Object, relation string, subject Subject) entryKey {
	return entryKey{Object: object, ID: object.ID, Relation: relation, Subject: subject}
}

type opKind uint8

const (
	opAdd opKind = iota
	opRemove
)

type change struct {
	revision Revision
	kind     opKind
	key      entryKey
}

// Store is a versioned, in-memory tuple store.
type Store struct {
	mu       sync.RWMutex
	changes  []change
	current  map[entryKey]struct{}
	revision Revision
	gcFloor  Revision
}

// NewStore returns an empty store at revision zero.
func NewStore() *Store {
	return &Store{current: make(map[entryKey]struct{})}
}

func (s *Store) validate(object Object, relation string, subject Subject) error {
	if object.IsZero() {
		return fmt.Errorf("tuple: empty object")
	}
	if !validNamespace(object.Namespace) || object.ID == "" {
		return fmt.Errorf("tuple: invalid object %q", object.String())
	}
	if !validRelationName(relation) {
		return fmt.Errorf("tuple: invalid relation %q", relation)
	}
	if subject.Namespace == "" || subject.ID == "" {
		return fmt.Errorf("tuple: empty subject")
	}
	return nil
}

// Write records that subject holds relation on object. Writing an entry that
// is already present is a no-op and leaves the revision unchanged.
func (s *Store) Write(object Object, relation string, subject Subject) (Revision, error) {
	if err := s.validate(object, relation, subject); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := newEntryKey(object, relation, subject)
	if _, ok := s.current[key]; ok {
		return s.revision, nil
	}
	s.revision++
	s.current[key] = struct{}{}
	s.changes = append(s.changes, change{revision: s.revision, kind: opAdd, key: key})
	return s.revision, nil
}

// Delete removes subject's relation on object. Deleting an entry that is not
// present is a no-op and leaves the revision unchanged.
func (s *Store) Delete(object Object, relation string, subject Subject) (Revision, error) {
	if err := s.validate(object, relation, subject); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := newEntryKey(object, relation, subject)
	if _, ok := s.current[key]; !ok {
		return s.revision, nil
	}
	s.revision++
	delete(s.current, key)
	s.changes = append(s.changes, change{revision: s.revision, kind: opRemove, key: key})
	return s.revision, nil
}

// Revision reports the store's current revision.
func (s *Store) Revision() Revision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revision
}

// GCFloor reports the oldest revision that can still be materialised.
func (s *Store) GCFloor() Revision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gcFloor
}

// CollectGarbage advances the GC floor to revision. Snapshots strictly older
// than the new floor can no longer be materialised. The current revision and
// the latest view are unaffected.
func (s *Store) CollectGarbage(revision Revision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if revision > s.revision {
		return fmt.Errorf("tuple: cannot collect to revision %d beyond %d", revision, s.revision)
	}
	if revision > s.gcFloor {
		s.gcFloor = revision
	}
	return nil
}

// View materialises the view at the requested revision. A zero revision means
// the latest view. Views are independent copies; later writes do not affect
// them.
func (s *Store) View(revision Revision) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if revision > s.revision {
		return nil, ErrFutureRevision
	}
	if revision != 0 && revision < s.gcFloor {
		return nil, ErrStaleRevision
	}
	if revision == 0 {
		revision = s.revision
	}
	return s.viewLocked(revision), nil
}

// viewLocked materialises the view at revision. The caller must hold at
// least a read lock.
func (s *Store) viewLocked(revision Revision) *Snapshot {
	index := make(map[Object]map[string][]Subject)
	for _, ch := range s.changes {
		if ch.revision > revision {
			break
		}
		switch ch.kind {
		case opAdd:
			relations := index[ch.key.Object]
			if relations == nil {
				relations = make(map[string][]Subject)
				index[ch.key.Object] = relations
			}
			relations[ch.key.Relation] = append(relations[ch.key.Relation], ch.key.Subject)
		case opRemove:
			relations := index[ch.key.Object]
			if relations == nil {
				continue
			}
			subjects := relations[ch.key.Relation]
			for i, existing := range subjects {
				if existing == ch.key.Subject {
					subjects = append(subjects[:i], subjects[i+1:]...)
					break
				}
			}
			if len(subjects) == 0 {
				delete(relations, ch.key.Relation)
				if len(relations) == 0 {
					delete(index, ch.key.Object)
				}
			} else {
				relations[ch.key.Relation] = subjects
			}
		}
	}

	return &Snapshot{revision: revision, index: index}
}

// Latest materialises the newest view.
func (s *Store) Latest() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.viewLocked(s.revision)
}

// Snapshot is an immutable view of the store at a single revision.
type Snapshot struct {
	revision Revision
	index    map[Object]map[string][]Subject
}

// Revision reports the revision the snapshot was materialised at.
func (s *Snapshot) Revision() Revision {
	if s == nil {
		return 0
	}
	return s.revision
}

// Subjects returns the subjects attached to relation on object, in insertion
// order. The returned slice must not be modified.
func (s *Snapshot) Subjects(object Object, relation string) []Subject {
	if s == nil {
		return nil
	}
	relations, ok := s.index[object]
	if !ok {
		return nil
	}
	return relations[relation]
}

// HasRelation reports whether relation on object has at least one tuple.
func (s *Snapshot) HasRelation(object Object, relation string) bool {
	return len(s.Subjects(object, relation)) > 0
}

// Relations lists the relations defined by tuples on object, sorted.
func (s *Snapshot) Relations(object Object) []string {
	if s == nil {
		return nil
	}
	relations, ok := s.index[object]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(relations))
	for name := range relations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Objects lists every object present in the snapshot, sorted.
func (s *Snapshot) Objects() []Object {
	if s == nil {
		return nil
	}
	objects := make([]Object, 0, len(s.index))
	for object := range s.index {
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].Namespace != objects[j].Namespace {
			return objects[i].Namespace < objects[j].Namespace
		}
		return objects[i].ID < objects[j].ID
	})
	return objects
}

// Clone returns an independent copy of the snapshot.
func (s *Snapshot) Clone() *Snapshot {
	if s == nil {
		return nil
	}
	index := make(map[Object]map[string][]Subject, len(s.index))
	for object, relations := range s.index {
		copied := make(map[string][]Subject, len(relations))
		for relation, subjects := range relations {
			copied[relation] = append([]Subject(nil), subjects...)
		}
		index[object] = copied
	}
	return &Snapshot{revision: s.revision, index: index}
}
