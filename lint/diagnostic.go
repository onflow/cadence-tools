package lint

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

type diagnostic struct {
	diagnostic analysis.Diagnostic
	report     func(analysis.Diagnostic)
}

func newDiagnostic(
	location common.Location,
	report func(analysis.Diagnostic),
	message string,
	position ast.Range,
) *diagnostic {
	return &diagnostic{
		diagnostic: analysis.Diagnostic{
			Location:       location,
			SuggestedFixes: []analysis.SuggestedFix{},
			Range:          position,
			Message:        message,
		},
		report: report,
	}
}

func (d *diagnostic) WithCode(code string) *diagnostic {
	d.diagnostic.Code = code
	return d
}

func (d *diagnostic) WithURL(url string) *diagnostic {
	d.diagnostic.URL = url
	return d
}

func (d *diagnostic) WithCategory(category string) *diagnostic {
	d.diagnostic.Category = category
	return d
}

func (d *diagnostic) WithSimpleReplacement(replacement string) *diagnostic {
	message := "Replace with `" + replacement + "`"
	if replacement == "" {
		message = "Remove code"
	}

	suggestedFix := analysis.SuggestedFix{
		Message: message,
		TextEdits: []ast.TextEdit{
			{
				Range:       ast.NewRangeFromPositioned(nil, d.diagnostic.Range),
				Replacement: replacement,
			},
		},
	}

	d.diagnostic.SuggestedFixes = append(d.diagnostic.SuggestedFixes, suggestedFix)
	return d
}

func (d *diagnostic) Report() {
	d.report(d.diagnostic)
}
