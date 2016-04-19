package quantizer

import (
	"github.com/DataDog/raclette/model"

	log "github.com/cihub/seelog"
)

func QuantizeCassandra(span model.Span) model.Span {
	log.Debug("Quantizing Cassandra span")

	// TODO(aaditya): just an alias for now, let's see if we need any special sauce
	return QuantizeSQL(span)
}
