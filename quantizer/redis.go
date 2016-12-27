package quantizer

import (
	"bytes"
	"sort"
	"strings"

	"github.com/DataDog/datadog-trace-agent/model"
)

// truncationMark is used as suffix by tracing libraries to indicate that a
// command was truncated.
const truncationMark = "..."

// Redis commands consisting in 2 words
var redisCompoundCommandSet = map[string]bool{
	"CLIENT": true, "CLUSTER": true, "COMMAND": true, "CONFIG": true, "DEBUG": true, "SCRIPT": true}

// QuantizeRedis generates resource for Redis spans
func QuantizeRedis(span model.Span) model.Span {
	query := compactWhitespaces(span.Resource)

	lines := []string{}
	for len(query) > 0 {
		var rawLine string

		idx := strings.IndexByte(query, '\n')
		if idx == -1 {
			rawLine = query
			query = ""
		} else {
			rawLine = query[:idx]
			query = query[idx+1:]
		}

		if line := strings.Trim(rawLine, " "); len(line) > 0 {
			lines = append(lines, string(line))
		}
	}

	readLine := func(line string) string {
		args := strings.SplitN(line, " ", 3)

		if strings.HasSuffix(args[0], truncationMark) {
			return ""
		}

		command := strings.ToUpper(args[0])

		if redisCompoundCommandSet[command] {
			command += " " + strings.ToUpper(args[1])
		}

		return command
	}

	var resource bytes.Buffer

	switch len(lines) {
	case 0:
		break

	case 1:
		// Single command
		if cmd := readLine(lines[0]); cmd != "" {
			resource.WriteString(cmd)
		}

	default:
		// Pipeline
		commandMap := make(map[string]struct{})

		for _, line := range lines {
			if cmd := readLine(line); cmd != "" {
				commandMap[cmd] = struct{}{}
			}
		}

		commands := make([]string, 0, len(commandMap))
		for command := range commandMap {
			commands = append(commands, command)
		}
		sort.Strings(commands)

		resource.WriteString("PIPELINE [")
		for _, command := range commands {
			resource.WriteByte(' ')
			resource.WriteString(command)
		}
		resource.WriteString(" ]")
	}

	span.Resource = strings.Trim(resource.String(), " ")

	return span
}
