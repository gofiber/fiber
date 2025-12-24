package logger

import (
	"bytes"
	"fmt"

	"github.com/gofiber/utils/v2"
)

// buildLogFuncChain analyzes the template and creates slices with the functions for execution and
// slices with the fixed parts of the template and the parameters
//
// fixParts contains the fixed parts of the template or parameters if a function is stored in the funcChain at this position
// funcChain contains for the parts which exist the functions for the dynamic parts
// funcChain and fixParts always have the same length and contain nil for the parts where no data is required in the chain,
// if a function exists for the part, a parameter for it can also exist in the fixParts slice
func buildLogFuncChain(cfg *Config, tagFunctions map[string]LogFunc) ([][]byte, []LogFunc, error) {
	// process flow is copied from the fasttemplate flow https://github.com/valyala/fasttemplate/blob/2a2d1afadadf9715bfa19683cdaeac8347e5d9f9/template.go#L23-L62
	templateB := utils.UnsafeBytes(cfg.Format)
	startTagB := utils.UnsafeBytes(startTag)
	endTagB := utils.UnsafeBytes(endTag)
	paramSeparatorB := utils.UnsafeBytes(paramSeparator)

	var fixParts [][]byte
	var funcChain []LogFunc

	for {
		before, after, found := bytes.Cut(templateB, startTagB)
		if !found {
			// no starting tag found in the existing template part
			break
		}
		// add fixed part
		funcChain = append(funcChain, nil)
		fixParts = append(fixParts, before)

		templateB = after
		before, after, found = bytes.Cut(templateB, endTagB)
		if !found {
			// cannot find end tag - just write it to the output.
			funcChain = append(funcChain, nil)
			fixParts = append(fixParts, startTagB)
			break
		}
		// ## function block ##
		// first check for tags with parameters
		tag, param, foundParam := bytes.Cut(before, paramSeparatorB)
		if foundParam {
			logFunc, ok := tagFunctions[utils.UnsafeString(tag)+paramSeparator]
			if !ok {
				return nil, nil, fmt.Errorf("%w: %q", ErrTemplateParameterMissing, utils.UnsafeString(before))
			}
			funcChain = append(funcChain, logFunc)
			// add param to the fixParts
			fixParts = append(fixParts, param)
		} else if logFunc, ok := tagFunctions[utils.UnsafeString(before)]; ok {
			// add functions without parameter
			funcChain = append(funcChain, logFunc)
			fixParts = append(fixParts, nil)
		}
		// ## function block end ##

		// reduce the template string
		templateB = after
	}
	// set the rest
	funcChain = append(funcChain, nil)
	fixParts = append(fixParts, templateB)

	return fixParts, funcChain, nil
}
