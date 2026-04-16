package worker

import (
	"context"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/funai-wiki/funai-chain/x/worker/client/cli"
	"github.com/funai-wiki/funai-chain/x/worker/keeper"
	"github.com/funai-wiki/funai-chain/x/worker/types"

	vrftypes "github.com/funai-wiki/funai-chain/x/vrf/types"
)

var (
	_ module.AppModuleBasic  = AppModuleBasic{}
	_ module.AppModule       = AppModule{}
	_ module.HasABCIEndBlock = AppModule{}
)

// VRFKeeper is the interface for VRF committee selection.
// P0-3: worker module uses VRF to select 100-person validator committee.
type VRFKeeper interface {
	SelectCommittee(ctx sdk.Context, eligibleWorkers []string) (vrftypes.CommitteeInfo, error)
}

// -------- AppModuleBasic --------

// AppModuleBasic implements the AppModuleBasic interface for the worker module.
type AppModuleBasic struct{}

func (AppModuleBasic) Name() string {
	return types.ModuleName
}

func (AppModuleBasic) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&types.MsgRegisterWorker{},
		&types.MsgExitWorker{},
		&types.MsgUpdateModels{},
		&types.MsgStake{},
		&types.MsgUnjail{},
	)
}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesis()
	bz, _ := json.Marshal(gs)
	return bz
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// -------- AppModule --------

// AppModule implements the AppModule interface for the worker module.
type AppModule struct {
	AppModuleBasic
	keeper    keeper.Keeper
	vrfKeeper VRFKeeper
}

// NewAppModule creates a new AppModule instance.
// P0-3: accepts optional VRFKeeper for committee selection.
func NewAppModule(k keeper.Keeper, vrfKeeper ...VRFKeeper) AppModule {
	m := AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         k,
	}
	if len(vrfKeeper) > 0 {
		m.vrfKeeper = vrfKeeper[0]
	}
	return m
}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg, keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg, keeper.NewQueryServerImpl(am.keeper))
}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if err := json.Unmarshal(data, &gs); err != nil {
		panic(fmt.Sprintf("failed to unmarshal %s genesis state: %s", types.ModuleName, err))
	}
	am.keeper.InitGenesis(ctx, gs)
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, _ := json.Marshal(gs)
	return bz
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

func (am AppModule) IsOnePerModuleType() {}
func (am AppModule) IsAppModule()        {}

// BeginBlock handles reputation decay every ~720 blocks (1 hour at 5s/block).
// Audit KT §3: all active workers' reputation decays toward 1.0 by ±0.005 per hour.
func (am AppModule) BeginBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()
	// 720 blocks = 1 hour at 5s/block
	if height%720 == 0 {
		am.keeper.ReputationDecayAll(sdkCtx)
	}
	return nil
}

// EndBlock computes validator updates using VRF 100-person committee every 120 blocks.
// P0-3: uses VRF SelectCommittee instead of returning all active workers.
// P0-4: uses Secp256k1 validator updates (matching Worker key type).
func (am AppModule) EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	am.keeper.ProcessExitingWorkers(sdkCtx)

	height := sdkCtx.BlockHeight()
	rotationPeriod := int64(120)
	if height%rotationPeriod != 0 {
		return nil, nil
	}

	workers := am.keeper.GetAllWorkers(sdkCtx)

	// Collect eligible (active, not jailed/tombstoned) workers
	var eligible []string
	workerPubkeys := make(map[string]string) // address → pubkey
	workerStakes := make(map[string]int64)   // address → stake power
	for _, w := range workers {
		if !w.IsActive() || w.Jailed || w.Tombstoned {
			continue
		}
		if len(w.Pubkey) == 0 {
			continue
		}
		eligible = append(eligible, w.Address)
		workerPubkeys[w.Address] = w.Pubkey
		power := w.Stake.Amount.Int64()
		if power <= 0 {
			power = 1
		}
		workerStakes[w.Address] = power
	}

	// P0-3: Use VRF to select 100-person committee if VRF keeper is available
	var selectedAddrs []string
	if am.vrfKeeper != nil && len(eligible) > 0 {
		committee, err := am.vrfKeeper.SelectCommittee(sdkCtx, eligible)
		if err == nil && len(committee.Members) > 0 {
			for _, m := range committee.Members {
				selectedAddrs = append(selectedAddrs, m.Address)
			}
		} else {
			// Fallback to all eligible if VRF fails
			selectedAddrs = eligible
		}
	} else {
		selectedAddrs = eligible
	}

	var updates []abci.ValidatorUpdate
	for _, addr := range selectedAddrs {
		pubkeyStr, ok := workerPubkeys[addr]
		if !ok || len(pubkeyStr) == 0 {
			continue
		}
		power := workerStakes[addr]
		// P0-4: Use secp256k1 key type to match Worker key type
		updates = append(updates, abci.UpdateValidator([]byte(pubkeyStr), power, "secp256k1"))
	}

	return updates, nil
}
