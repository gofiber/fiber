package logger

import (
	"bytes"
	"errors"

	"github.com/gofiber/fiber/v2/utils"
)

func buildLogFuncChain(cfg *Config, tagFunctions map[string]LogFunc) (templateChain [][]byte, logChain []LogFunc, err error) {
	templateB := utils.UnsafeBytes(cfg.Format)
	startTagB := utils.UnsafeBytes(startTag)
	endTagB := utils.UnsafeBytes(endTag)
	paramSeparatorB := utils.UnsafeBytes(paramSeparator)

	for {
		currentPos := bytes.Index(templateB, startTagB)
		if currentPos < 0 {
			break
		}
		logChain = append(logChain, nil)
		templateChain = append(templateChain, templateB[:currentPos])

		templateB = templateB[currentPos+len(startTagB):]
		currentPos = bytes.Index(templateB, endTagB)
		if currentPos < 0 {
			// cannot find end tag - just write it to the output.
			logChain = append(logChain, nil)
			templateChain = append(templateChain, startTagB)
			break
		}
		// first check for tags with parameters
		if index := bytes.Index(templateB[:currentPos], paramSeparatorB); index != -1 {
			if logFunc, ok := tagFunctions[utils.UnsafeString(templateB[:index+1])]; ok {
				logChain = append(logChain, logFunc)
				templateChain = append(templateChain, templateB[index+1:currentPos])
			} else {
				return nil, nil, errors.New("No parameter found in \"" + utils.UnsafeString(templateB[:currentPos]) + "\"")
			}
		} else if logFunc, ok := tagFunctions[utils.UnsafeString(templateB[:currentPos])]; ok {
			logChain = append(logChain, logFunc)
			templateChain = append(templateChain, nil)
		}
		templateB = templateB[currentPos+len(endTagB):]
	}
	logChain = append(logChain, nil)
	templateChain = append(templateChain, templateB)

	return
}
