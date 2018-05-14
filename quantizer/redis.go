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
func QuantizeRedis(span *model.Span) {
	query := compactWhitespaces(span.Resource)

	var resource bytes.Buffer
	truncated := false
	nbCmds := 0

	for len(query) > 0 && nbCmds < maxRedisNbCommands {
		var rawLine string

		// Read the next command
		idx := strings.IndexByte(query, '\n')
		if idx == -1 {
			rawLine = query
			query = ""
		} else {
			rawLine = query[:idx]
			query = query[idx+1:]
		}

		line := strings.Trim(rawLine, " ")
		if len(line) == 0 {
			continue
		}

		// Parse arguments
		args := strings.SplitN(line, " ", 3)

		if strings.HasSuffix(args[0], redisTruncationMark) {
			truncated = true
			continue
		}

		command := strings.ToUpper(args[0])

		if redisCompoundCommandSet[command] && len(args) > 1 {
			if strings.HasSuffix(args[1], redisTruncationMark) {
				truncated = true
				continue
			}

			command += " " + strings.ToUpper(args[1])
		}

		// Write the command representation
		resource.WriteByte(' ')
		resource.WriteString(command)

		nbCmds++
		truncated = false
	}

	if nbCmds == maxRedisNbCommands || truncated {
		resource.WriteString(" ...")
	}

	span.Resource = strings.Trim(resource.String(), " ")
}
