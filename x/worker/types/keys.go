package types

const (
	ModuleName = "worker"

	StoreKey = ModuleName

	RouterKey = ModuleName

	QuerierRoute = ModuleName

	// ModuleAccountName is the module account used to hold worker deposits.
	ModuleAccountName = ModuleName
)

var (
	// WorkerKeyPrefix stores Worker structs keyed by address bytes.
	WorkerKeyPrefix = []byte{0x01}

	// ModelIndexPrefix stores model→worker mappings: 0x02 | model_id | 0x00 | addr_bytes.
	ModelIndexPrefix = []byte{0x02}

	// ParamsKey is the key for module parameters.
	ParamsKey = []byte{0x03}
)

// WorkerKey returns the store key for a specific worker.
func WorkerKey(addr []byte) []byte {
	return append(WorkerKeyPrefix, addr...)
}

// ModelIndexKey returns the composite key for a model→worker index entry.
func ModelIndexKey(modelID string, addr []byte) []byte {
	key := make([]byte, 0, len(ModelIndexPrefix)+len(modelID)+1+len(addr))
	key = append(key, ModelIndexPrefix...)
	key = append(key, []byte(modelID)...)
	key = append(key, 0x00) // separator
	key = append(key, addr...)
	return key
}

// ModelIndexIteratorPrefix returns the prefix for iterating all workers of a model.
func ModelIndexIteratorPrefix(modelID string) []byte {
	key := make([]byte, 0, len(ModelIndexPrefix)+len(modelID)+1)
	key = append(key, ModelIndexPrefix...)
	key = append(key, []byte(modelID)...)
	key = append(key, 0x00)
	return key
}
