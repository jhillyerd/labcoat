package runner

import "strings"

const (
	labelStart = "[label{{{"
	labelEnd   = "}}}label]"
)

func NewScript(cmds []string) string {
	result := ""

	for _, cmd := range cmds {
		result += "echo \"" + labelStart + escape(cmd) + labelEnd + "\"\n"
		result += cmd + "\n\n"
	}

	return result
}

func escape(cmd string) string {
	return strings.ReplaceAll(cmd, "\"", "\\\"")
}

func FormatOutput(s string, labelFn func(string) string) string {
	var out string

	startLen := len(labelStart)
	endLen := len(labelEnd)

	for {
		start := strings.Index(s, labelStart)
		if start == -1 {
			break
		}

		if start > 0 {
			out += s[:start]
		}
		s = s[start+startLen:]

		label := s
		end := strings.Index(s, labelEnd)
		if end == -1 {
			// No end marker; render the rest of `s` as label.
			s = ""
		} else {
			label = s[:end]
			s = s[end+endLen:]
		}
		out += labelFn(label)
	}

	out += s

	return out
}
