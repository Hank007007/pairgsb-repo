// Package fault defines the reasons an extraction can be refused.
//
// Each reason is a distinct sentinel so callers can tell why a package was
// rejected instead of receiving one generic failure.
package fault

import "errors"

var (
	// ErrUnsafeName reports an entry name that the policy does not permit.
	ErrUnsafeName = errors.New("unsafe entry name")

	// ErrOutsideRoot reports a resolved location that is not under the job root.
	ErrOutsideRoot = errors.New("entry outside job root")

	// ErrSymlinkEscape reports a symlink whose target is not under the job root.
	ErrSymlinkEscape = errors.New("symlink target outside job root")

	// ErrHardlinkEscape reports a hardlink whose target is not under the job root.
	ErrHardlinkEscape = errors.New("hardlink target outside job root")

	// ErrUnsupportedType reports an entry type the extractor does not accept.
	ErrUnsupportedType = errors.New("unsupported entry type")

	// ErrBudgetExceeded reports that the written-byte budget was reached.
	ErrBudgetExceeded = errors.New("size budget exceeded")

	// ErrEntryLimit reports that the entry-count limit was reached.
	ErrEntryLimit = errors.New("entry limit exceeded")

	// ErrRatioExceeded reports that the compression ratio limit was reached.
	ErrRatioExceeded = errors.New("compression ratio exceeded")
)
