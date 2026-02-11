package lint

import (
	"fmt"
	"sort"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/tools/analysis"
)

type CodeManager interface {
	ReadCode(location common.Location) ([]byte, error)
	WriteCode(location common.Location, code []byte) error
}

func ApplyFixes(diagnostics []analysis.Diagnostic, codeManager CodeManager) error {

	// Group diagnostics by location
	diagnosticsByLocation := make(map[common.Location][]analysis.Diagnostic)
	for _, diag := range diagnostics {
		diagnosticsByLocation[diag.Location] = append(diagnosticsByLocation[diag.Location], diag)
	}

	// Process each location
	for location, diags := range diagnosticsByLocation {
		err := ApplyFixesInLocation(location, diags, codeManager)
		if err != nil {
			return fmt.Errorf("failed to apply fixes for location %v: %w", location, err)
		}
	}

	return nil
}

// filterOverlappingEdits removes overlapping edits, keeping the first occurrence
func filterOverlappingEdits(edits []ast.TextEdit) []ast.TextEdit {
	if len(edits) <= 1 {
		return edits
	}

	result := make([]ast.TextEdit, 0, len(edits))
	for i, edit := range edits {
		overlaps := false
		for j := 0; j < i; j++ {
			if editsOverlap(edits[j], edit) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			result = append(result, edit)
		}
	}
	return result
}

func ApplyFixesInLocation(
	location common.Location,
	diagnostics []analysis.Diagnostic,
	codeManager CodeManager,
) error {
	// Collect all edits from first suggested fix of each diagnostic
	var allEdits []ast.TextEdit
	for _, diag := range diagnostics {
		if len(diag.SuggestedFixes) > 0 {
			// Use only the first suggested fix (typically the recommended one)
			firstFix := diag.SuggestedFixes[0]
			allEdits = append(allEdits, firstFix.TextEdits...)
		}
	}

	if len(allEdits) == 0 {
		return nil
	}

	// Filter out overlapping edits
	nonOverlappingEdits := filterOverlappingEdits(allEdits)
	if len(nonOverlappingEdits) == 0 {
		return nil
	}

	// Read original code
	code, err := codeManager.ReadCode(location)
	if err != nil {
		return fmt.Errorf("failed to get code for location %v: %w", location, err)
	}

	// Sort edits by offset in descending order (highest offset first)
	sort.Slice(nonOverlappingEdits, func(i, j int) bool {
		offset1 := nonOverlappingEdits[i].Range.StartPos.Offset
		offset2 := nonOverlappingEdits[j].Range.StartPos.Offset
		return offset1 > offset2
	})

	// Apply edits sequentially, in reverse order (see sorting above)
	modifiedCode := string(code)
	for _, edit := range nonOverlappingEdits {
		modifiedCode = edit.ApplyTo(modifiedCode)
	}

	// Write modified code back
	err = codeManager.WriteCode(location, []byte(modifiedCode))
	if err != nil {
		return fmt.Errorf("failed to set fixed code for location %v: %w", location, err)
	}

	return nil
}

// editsOverlap returns true if the two edits overlap in their ranges,
// false otherwise
func editsOverlap(a, b ast.TextEdit) bool {
	aStart := a.Range.StartPos.Offset
	aEnd := a.Range.EndPos.Offset
	bStart := b.Range.StartPos.Offset
	bEnd := b.Range.EndPos.Offset

	// Two ranges overlap if:
	// - A's end is after B's start, AND
	// - B's end is after A's start
	return aEnd > bStart && bEnd > aStart
}
