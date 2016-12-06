package quantizer

import (
	"strings"

	"github.com/DataDog/datadog-trace-agent/model"
)

// Redis commands consisting in 2 words
var redisCompoundCommandSet = map[string]bool{
	"CLIENT": true, "CLUSTER": true, "COMMAND": true, "CONFIG": true, "DEBUG": true, "SCRIPT": true}

// QuantizeRedis generates resource for Redis spans
func QuantizeRedis(span model.Span) model.Span {
	query := span.Resource

	resource := ""
	previousCommand := ""
	previousDuplicate := false

	query = compactWhitespaces(query)
	lines := strings.Split(query, "\n")

	for _, q := range lines {
		q = strings.Trim(q, " ")
		if len(q) > 0 {
			args := strings.SplitN(q, " ", 3)
			command := strings.ToUpper(args[0])
			if redisCompoundCommandSet[command] {
				command += " " + strings.ToUpper(args[1])
			}

			if command == previousCommand {
				if !previousDuplicate {
					resource += "*"
					previousDuplicate = true
				}
			} else {
				resource += " " + command
				previousCommand = command
				previousDuplicate = false
			}
		}
	}

	span.Resource = strings.Trim(resource, " ")

	return span
}
