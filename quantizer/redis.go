package quantizer

import (
	"bytes"
	"strings"

	"github.com/DataDog/datadog-trace-agent/model"
)

// redisTruncationMark is used as suffix by tracing libraries to indicate that a
// command was truncated.
const redisTruncationMark = "..."

const maxRedisNbCommands = 3

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

	isArgTruncated := func(arg string) bool {
		return strings.HasSuffix(arg, redisTruncationMark)
	}

	readLine := func(line string) string {
		args := strings.SplitN(line, " ", 3)

		// Ignore truncated commands
		if isArgTruncated(args[0]) {
			return ""
		}

		command := strings.ToUpper(args[0])

		if redisCompoundCommandSet[command] {
			if isArgTruncated(args[1]) {
				return ""
			}

			command += " " + strings.ToUpper(args[1])
		}

		return command
	}

	var resource bytes.Buffer
	var prevCmd string

	multipleCmds := false
	nbCmds := 0
	truncatedLastLine := false

	for i, line := range lines {
		cmd := readLine(line)
		if cmd == "" {
			if i == len(lines)-1 {
				truncatedLastLine = true
			}
			continue
		}

		if cmd == prevCmd {
			if !multipleCmds {
				resource.WriteByte('*')
			}

			multipleCmds = true
		} else {
			resource.WriteByte(' ')
			resource.WriteString(cmd)

			nbCmds++
			if nbCmds == maxRedisNbCommands {
				break
			}

			multipleCmds = false
		}

		prevCmd = cmd
	}

	if nbCmds == maxRedisNbCommands || truncatedLastLine {
		resource.WriteString(" ...")
	}

	span.Resource = strings.Trim(resource.String(), " ")

	return span
}
