package types

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		InitialSeed: VRFSeed{Value: []byte("funai-genesis-seed-v1"), BlockHeight: 0},
		Leaders:     []LeaderInfo{},
		Committee:   nil,
	}
}

type GenesisState struct {
	Params      Params         `json:"params"`
	InitialSeed VRFSeed        `json:"initial_seed"`
	Leaders     []LeaderInfo   `json:"leaders"`
	Committee   *CommitteeInfo `json:"committee,omitempty"`
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	if len(gs.InitialSeed.Value) == 0 {
		return ErrInvalidGenesis.Wrap("initial seed cannot be empty")
	}
	return nil
}
