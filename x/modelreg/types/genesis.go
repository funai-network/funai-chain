package types

// GenesisState defines the modelreg module's genesis state.
type GenesisState struct {
	Params Params  `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	Models []Model `protobuf:"bytes,2,rep,name=models,proto3" json:"models"`
}

func (m *GenesisState) ProtoMessage()  {}
func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return "modelreg.GenesisState" }

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
		Models: []Model{},
	}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, m := range gs.Models {
		if m.ModelId == "" {
			return ErrInvalidModelId
		}
		if seen[m.ModelId] {
			return ErrModelAlreadyExists
		}
		seen[m.ModelId] = true
	}
	return nil
}
