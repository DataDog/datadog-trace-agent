package fixtures

import (
	"fmt"
	"math/rand"

	"github.com/StackVista/stackstate-trace-agent/model"
)

// RandomServices generates random services metadata
func RandomServices(maxServices, maxTags int) model.ServicesMetadata {
	services := make(map[string]map[string]string)

	k := 0
	nbServices := 1 + rand.Intn(maxServices-1)
	for i := 0; i < nbServices; i++ {
		service := fmt.Sprintf("service%03d", i)
		services[service] = make(map[string]string)
		nbTags := 1 + rand.Intn(maxTags-1)
		for j := 0; j < nbTags; j++ {
			key := fmt.Sprintf("key%05d", k)
			value := fmt.Sprintf("value%04d", k)
			services[service][key] = value
			k++
		}
	}

	return services
}
