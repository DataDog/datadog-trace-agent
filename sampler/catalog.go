package sampler

import (
	"stackstate-trace-agent/model"
)

const defaultServiceRateKey = "service:,env:"

type serviceKeyCatalog map[string]Signature

func byServiceKey(service, env string) string {
	return "service:" + service + ",env:" + env
}

func newServiceKeyCatalog() serviceKeyCatalog {
	return serviceKeyCatalog(make(map[string]Signature))
}

func (cat serviceKeyCatalog) register(root *model.Span, env string, sig Signature) {
	map[string]Signature(cat)[byServiceKey(root.Service, env)] = sig
}

func (cat serviceKeyCatalog) getRateByService(rates map[Signature]float64, totalScore float64) map[string]float64 {
	rbs := make(map[string]float64, len(rates)+1)
	for key, sig := range map[string]Signature(cat) {
		if rate, ok := rates[sig]; ok {
			rbs[key] = rate
		} else {
			// Backend, with its decay mecanism, should automatically remove the entries
			// which have such a low value that they don't matter any more.
			delete(cat, key)
		}
	}
	rbs[defaultServiceRateKey] = totalScore
	return rbs
}
