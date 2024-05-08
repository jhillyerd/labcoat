package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatOutput(t *testing.T) {
	renderFn := func(s string) string {
		return "|:" + s + ":|"
	}

	tcs := map[string]struct {
		in, want string
	}{
		"empty str":    {in: "", want: ""},
		"plain str":    {in: "hello world", want: "hello world"},
		"naked":        {in: "[label{{{naked}}}label]", want: "|:naked:|"},
		"empty":        {in: "[label{{{}}}label]", want: "|::|"},
		"simple":       {in: "abc[label{{{naked}}}label]def", want: "abc|:naked:|def"},
		"spaces":       {in: "abc [label{{{two words}}}label] def", want: "abc |:two words:| def"},
		"nlsuffix":     {in: "abc[label{{{naked}}}label]\ndef", want: "abc|:naked:|\ndef"},
		"nllabel":      {in: "abc[label{{{new\nline}}}label]def", want: "abc|:new\nline:|def"},
		"unterminated": {in: "abc [label{{{no term!", want: "abc |:no term!:|"},
	}
	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := FormatOutput(tc.in, renderFn)

			assert.Equal(t, tc.want, got)
		})
	}
}
