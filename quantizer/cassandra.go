package quantizer

import (
	"github.com/DataDog/raclette/model"
)

func QuantizeCassandra(span model.Span) model.Span {
	// TODO(aaditya): just an alias for now, let's see if we need any special sauce
	return QuantizeSQL(span)
}
