package types

const (
	ModuleName   = "modelreg"
	StoreKey     = ModuleName
	RouterKey    = ModuleName
	QuerierRoute = ModuleName
)

var (
	ModelKeyPrefix             = []byte{0x01}
	ParamsKey                  = []byte{0x02}
	WorkerInstalledModelPrefix = []byte{0x03} // reverse index: worker → installed model_ids
	AliasIndexPrefix           = []byte{0x04} // alias → model_id reverse index
)

func ModelKey(modelID string) []byte {
	return append(ModelKeyPrefix, []byte(modelID)...)
}

// AliasKey returns the key for the alias → model_id reverse index.
func AliasKey(alias string) []byte {
	return append(AliasIndexPrefix, []byte(alias)...)
}

// WorkerInstalledModelKey returns the key for a worker's installed model entry.
// Layout: 0x03 | addr_bytes | 0x00 | model_id_bytes
func WorkerInstalledModelKey(workerAddr []byte, modelID string) []byte {
	key := make([]byte, 0, len(WorkerInstalledModelPrefix)+len(workerAddr)+1+len(modelID))
	key = append(key, WorkerInstalledModelPrefix...)
	key = append(key, workerAddr...)
	key = append(key, 0x00) // separator
	key = append(key, []byte(modelID)...)
	return key
}

// WorkerInstalledModelIteratorPrefix returns the prefix for iterating all models installed by a worker.
func WorkerInstalledModelIteratorPrefix(workerAddr []byte) []byte {
	key := make([]byte, 0, len(WorkerInstalledModelPrefix)+len(workerAddr)+1)
	key = append(key, WorkerInstalledModelPrefix...)
	key = append(key, workerAddr...)
	key = append(key, 0x00)
	return key
}
