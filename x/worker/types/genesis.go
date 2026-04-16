package types

// GenesisState defines the worker module's genesis state.
type GenesisState struct {
	Params  Params   `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	Workers []Worker `protobuf:"bytes,2,rep,name=workers,proto3" json:"workers"`
}

func (m *GenesisState) ProtoMessage()  {}
func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return "worker.GenesisState" }

// DefaultGenesis returns the default genesis state for the worker module.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:  DefaultParams(),
		Workers: []Worker{},
	}
}

// Validate performs basic genesis state validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, w := range gs.Workers {
		if seen[w.Address] {
			return ErrWorkerAlreadyRegistered
		}
		seen[w.Address] = true
	}
	return nil
}
