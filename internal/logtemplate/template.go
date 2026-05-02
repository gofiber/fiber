package logtemplate

import (
	"bytes"

	"github.com/gofiber/utils/v2"
)

const (
	startTag       = "${"
	endTag         = "}"
	paramSeparator = ":"
)

// Buffer is the minimal write surface required by template renderers.
// It is intentionally narrow (Write/WriteByte/WriteString plus Write so
// fmt.Fprintf works) so any byte-buffer implementation — including the
// pooled *bytebufferpool.ByteBuffer used internally — satisfies it without
// having to expose its full machinery.
type Buffer interface {
	Write(p []byte) (int, error)
	WriteByte(c byte) error
	WriteString(s string) (int, error)
}

// Func renders one dynamic template tag.
type Func[C, D any] func(output Buffer, ctx C, data *D, extraParam string) (int, error)

// Template is a precompiled log template.
type Template[C, D any] struct {
	fixedParts [][]byte
	funcChain  []Func[C, D]
}

// Build parses format once and returns a reusable template.
func Build[C, D any](format string, tagFunctions map[string]Func[C, D]) (*Template[C, D], error) {
	templateB := utils.UnsafeBytes(format)
	startTagB := utils.UnsafeBytes(startTag)
	endTagB := utils.UnsafeBytes(endTag)
	paramSeparatorB := utils.UnsafeBytes(paramSeparator)

	chainCapacity := 2*bytes.Count(templateB, startTagB) + 1
	fixedParts := make([][]byte, 0, chainCapacity)
	funcChain := make([]Func[C, D], 0, chainCapacity)

	for {
		before, after, found := bytes.Cut(templateB, startTagB)
		if !found {
			break
		}

		funcChain = append(funcChain, nil)
		fixedParts = append(fixedParts, before)

		templateB = after
		before, after, found = bytes.Cut(templateB, endTagB)
		if !found {
			funcChain = append(funcChain, nil)
			fixedParts = append(fixedParts, startTagB)
			break
		}

		tag, param, foundParam := bytes.Cut(before, paramSeparatorB)
		if foundParam {
			fn, ok := tagFunctions[utils.UnsafeString(tag)+paramSeparator]
			if !ok {
				return nil, &UnknownTagError{Tag: string(before), Param: string(param)}
			}
			funcChain = append(funcChain, fn)
			fixedParts = append(fixedParts, param)
		} else if fn, ok := tagFunctions[utils.UnsafeString(before)]; ok {
			funcChain = append(funcChain, fn)
			fixedParts = append(fixedParts, nil)
		} else {
			tagErr := &UnknownTagError{Tag: string(before)}
			// Common typo: user wrote ${reqHeader} when reqHeader: is registered
			// as a parametric tag. Surface the parametric form as a hint.
			if _, hasParametric := tagFunctions[utils.UnsafeString(before)+paramSeparator]; hasParametric {
				tagErr.Hint = `did you mean ${` + string(before) + `:PARAM}?`
			}
			return nil, tagErr
		}

		templateB = after
	}

	funcChain = append(funcChain, nil)
	fixedParts = append(fixedParts, templateB)

	return &Template[C, D]{
		fixedParts: fixedParts,
		funcChain:  funcChain,
	}, nil
}

// Chains returns the fixed template parts and functions used by Execute.
func (t *Template[C, D]) Chains() ([][]byte, []Func[C, D]) {
	if t == nil {
		return nil, nil
	}
	return t.fixedParts, t.funcChain
}

// Execute renders the template into output.
func (t *Template[C, D]) Execute(output Buffer, ctx C, data *D) error {
	if t == nil {
		return nil
	}
	return ExecuteChains(output, ctx, data, t.fixedParts, t.funcChain)
}

// ExecuteChains renders precompiled template chains into output.
func ExecuteChains[C, D any](output Buffer, ctx C, data *D, fixedParts [][]byte, funcChain []Func[C, D]) error {
	for i, fn := range funcChain {
		switch {
		case fn == nil:
			if _, err := output.Write(fixedParts[i]); err != nil {
				return err
			}
		case fixedParts[i] == nil:
			if _, err := fn(output, ctx, data, ""); err != nil {
				return err
			}
		default:
			if _, err := fn(output, ctx, data, utils.UnsafeString(fixedParts[i])); err != nil {
				return err
			}
		}
	}

	return nil
}
