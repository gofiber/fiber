package logger

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gofiber/utils"
)

// MIT License fasttemplate
// Copyright (c) 2015 Aliaksandr Valialkin
// https://github.com/valyala/fasttemplate/blob/master/LICENSE

type (
	loggerTemplate struct {
		template string
		startTag string
		endTag   string
		texts    [][]byte
		tags     []string
	}
	loggerTagFunc func(w io.Writer, tag string) (int, error)
)

func (t *loggerTemplate) new(template, startTag, endTag string) {
	t.template = template
	t.startTag = startTag
	t.endTag = endTag
	t.texts = t.texts[:0]
	t.tags = t.tags[:0]

	if len(startTag) == 0 {
		panic("startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("endTag cannot be empty")
	}

	s := utils.GetBytes(template)
	a := utils.GetBytes(startTag)
	b := utils.GetBytes(endTag)

	tagsCount := bytes.Count(s, a)
	if tagsCount == 0 {
		return
	}

	if tagsCount+1 > cap(t.texts) {
		t.texts = make([][]byte, 0, tagsCount+1)
	}
	if tagsCount > cap(t.tags) {
		t.tags = make([]string, 0, tagsCount)
	}

	for {
		n := bytes.Index(s, a)
		if n < 0 {
			t.texts = append(t.texts, s)
			break
		}
		t.texts = append(t.texts, s[:n])

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			panic(fmt.Errorf("cannot find end tag=%q in the template=%q starting from %q", endTag, template, s))
		}

		t.tags = append(t.tags, utils.GetString(s[:n]))
		s = s[n+len(b):]
	}
}

func (t *loggerTemplate) executeFunc(w io.Writer, f loggerTagFunc) (int64, error) {
	var nn int64

	n := len(t.texts) - 1
	if n == -1 {
		ni, err := w.Write(utils.GetBytes(t.template))
		return int64(ni), err
	}

	for i := 0; i < n; i++ {
		ni, err := w.Write(t.texts[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		ni, err = f(w, t.tags[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
	}
	ni, err := w.Write(t.texts[n])
	nn += int64(ni)
	return nn, err
}
