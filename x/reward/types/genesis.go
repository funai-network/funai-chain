package types

import "github.com/cosmos/gogoproto/proto"

func init() {
	proto.RegisterType((*GenesisState)(nil), "funai.reward.GenesisState")
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		RewardRecords: []RewardRecord{},
	}
}

type GenesisState struct {
	Params        Params         `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	RewardRecords []RewardRecord `protobuf:"bytes,2,rep,name=reward_records,proto3" json:"reward_records"`
}

func (m *GenesisState) ProtoMessage()  {}
func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return "reward.GenesisState" }

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	return nil
}
