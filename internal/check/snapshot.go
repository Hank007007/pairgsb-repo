package check

import (
	"tuplegrate/internal/schema"
	"tuplegrate/internal/tuple"
)

// View couples a tuple snapshot with the schema that interprets it. Every
// check evaluates against exactly one view.
type View struct {
	tuples *tuple.Snapshot
	schema *schema.Schema
}

func newView(tuples *tuple.Snapshot, rules *schema.Schema) *View {
	return &View{tuples: tuples, schema: rules}
}

// Revision reports the revision of the tuple snapshot backing the view.
func (v *View) Revision() tuple.Revision {
	if v == nil || v.tuples == nil {
		return 0
	}
	return v.tuples.Revision()
}

// AtRevision returns the view used by a check that carries a revision. A
// zero revision denotes the newest view.
func (s *Service) AtRevision(revision tuple.Revision) (*View, error) {
	snapshot, err := s.store.View(revision)
	if err != nil {
		switch err {
		case tuple.ErrFutureRevision:
			return nil, newError(CodeFutureRevision, "revision %d is newer than the store", revision)
		case tuple.ErrStaleRevision:
			return nil, newError(CodeStaleRevision, "revision %d has been garbage collected", revision)
		default:
			return nil, newError(CodeInvalidRequest, "%v", err)
		}
	}
	return newView(snapshot, s.rules), nil
}

// Latest returns a view over the store's newest revision.
func (s *Service) Latest() *View {
	return newView(s.store.Latest(), s.rules)
}
