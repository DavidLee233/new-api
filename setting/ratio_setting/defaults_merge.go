package ratio_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

func loadMapWithDefaultsAndCallback[K comparable, V any](m *types.RWMap[K, V], defaults map[K]V, jsonStr string, onSuccess func()) error {
	merged := make(map[K]V, len(defaults))
	for k, v := range defaults {
		merged[k] = v
	}

	if err := common.Unmarshal([]byte(jsonStr), &merged); err != nil {
		return err
	}

	m.Clear()
	m.AddAll(merged)
	if onSuccess != nil {
		onSuccess()
	}
	return nil
}
