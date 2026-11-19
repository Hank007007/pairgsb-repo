package check

import (
	"tuplegrate/internal/schema"
	"tuplegrate/internal/tuple"
)

// Request is a single permission check.
type Request struct {
	// Subject is the entity asking for access, in namespace:id or
	// namespace:id#relation form.
	Subject string

	// Relation is the relation being checked.
	Relation string

	// Object is the entity access is requested to, in namespace:id form.
	Object string

	// Revision selects the snapshot the check evaluates against. Zero means
	// the newest revision.
	Revision tuple.Revision
}

// Result is the outcome of a permitted check.
type Result struct {
	// Allowed reports whether the request subject is covered by relation on
	// object in the selected snapshot.
	Allowed bool

	// Revision is the snapshot revision the check actually evaluated
	// against.
	Revision tuple.Revision
}

// Service evaluates checks against a tuple store and a compiled schema.
type Service struct {
	store *tuple.Store
	rules *schema.Schema
}

// New returns a Service over store and rules. Both must be non-nil.
func New(store *tuple.Store, rules *schema.Schema) *Service {
	return &Service{store: store, rules: rules}
}

// Schema returns the compiled schema the service evaluates against.
func (s *Service) Schema() *schema.Schema {
	return s.rules
}

// Check reports whether req.Subject holds req.Relation on req.Object.
func (s *Service) Check(req Request) (Result, error) {
	subject, err := tuple.ParseSubject(req.Subject)
	if err != nil {
		return Result{}, newError(CodeInvalidRequest, "subject: %v", err)
	}
	object, err := tuple.ParseObject(req.Object)
	if err != nil {
		return Result{}, newError(CodeInvalidRequest, "object: %v", err)
	}
	if req.Relation == "" {
		return Result{}, newError(CodeInvalidRequest, "relation is empty")
	}

	view, err := s.AtRevision(req.Revision)
	if err != nil {
		return Result{}, err
	}
	result := Result{Revision: view.Revision()}

	rewrite, ok := view.schema.Relation(object.Namespace, req.Relation)
	if !ok {
		return result, newError(CodeUnknownRelation, "%s#%s is not declared", object.Namespace, req.Relation)
	}

	state := &evalState{view: view}
	allowed, err := state.evalRewrite(rewrite.Rewrite, object, req.Relation, subject, newContext())
	if err != nil {
		return result, err
	}
	result.Allowed = allowed
	return result, nil
}
