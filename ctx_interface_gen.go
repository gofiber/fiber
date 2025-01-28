//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func main() {
	const filename = "ctx_interface.go"

	// 1) read file
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("failed to read file: %w", err))
	}

	// 2) patch interface
	patched, err := patchCtxFile(data)
	if err != nil {
		panic(err)
	}

	// 3) write patched file
	if err := os.WriteFile(filename, patched, 0o644); err != nil {
		panic(fmt.Errorf("failed to write patched file: %w", err))
	}
}

// patchCtxFile adjust the Ctx interface in the given file
func patchCtxFile(input []byte) ([]byte, error) {
	// process file line by line
	in := bytes.NewReader(input)
	scanner := bufio.NewScanner(in)
	var outBuf bytes.Buffer

	regexCtx := regexp.MustCompile(`\bCtx\b`)
	regexApp := regexp.MustCompile(`\*\bApp\b`)

	for scanner.Scan() {
		line := scanner.Text()

		// A) change interface head definition
		//  => "type Ctx interface {" -> "type Ctx[T any] interface {"
		if strings.HasPrefix(line, "type") {
			line = strings.Replace(line,
				"type Ctx interface {",
				"type Ctx[T any] interface {",
				1,
			)
		} else {
			// B) replace every use of Ctx with T but only in the function definitions
			// via regex and boundary word matching
			//  => "func (app *App[TCtx]) newCtx() Ctx {" -> "func (app *App[TCtx]) newCtx() T {"
			if strings.Contains(line, "Ctx") {
				line = regexCtx.ReplaceAllString(line, "T")
			}

			// C) App with generic type
			if strings.Contains(line, "App") {
				// TODO: check this part
				line = regexApp.ReplaceAllString(line, "*App[T]")
			}
		}

		outBuf.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return outBuf.Bytes(), nil
}
