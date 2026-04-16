# FunAI Cosmos EVM Integration Engineering Guide

> Date: 2026-04-03
> Baseline: commit ce87883 (Cosmos SDK v0.50.10)
> Goal: Integrate EVM module on FunAI chain, supporting Solidity contract deployment and execution
> Document suffix: KT

---

## 1. Which Version to Use

**Cosmos EVM (formerly evmOS)**, Apache 2.0 license, maintained by Interchain Labs.

```
Repository: https://github.com/evmos/os (migrating to cosmos/evm soon)
License: Apache 2.0 (open-sourced in March 2025 with ICF funding)
Maintainer: Interchain Labs (official Cosmos SDK engineering team)
Status: Official EVM implementation for the Cosmos ecosystem, replacing the old Ethermint

Do NOT use:
  × github.com/evmos/ethermint — no longer maintained
  × github.com/cosmos/ethermint — even older deprecated version
```

### Version Compatibility

```
FunAI current: Cosmos SDK v0.50.10 + CometBFT v0.38.12

Need to confirm whether the latest release of Cosmos EVM (evmos/os) is compatible with SDK v0.50.x.
If it only supports v0.47.x or v0.51.x, options are:
  Option A: Upgrade FunAI to a matching SDK version
  Option B: Find a v0.50-compatible branch of Cosmos EVM
  Option C: Adapt it ourselves (fork + modify go.mod)

Run go mod tidy first to see if it errors out, then decide.
```

---

## 2. Engineering Implementation Steps

### Day 1: Add Dependencies + Get It Compiling

#### 2.1 Add Go Dependencies

```bash
cd ~/funai-chain

# Add Cosmos EVM dependency (exact version number depends on actual compatible version)
go get github.com/evmos/os@latest
# Or if already migrated to the cosmos org:
# go get cosmossdk.io/x/evm@latest

go mod tidy
```

If `go mod tidy` reports version conflicts, resolve them one by one — usually CometBFT or cosmos-proto version mismatches. Use `go mod edit -replace` to force specific versions.

#### 2.2 Register the EVM Module

Modify `app/app.go`:

```go
import (
    // Existing imports ...
    
    // Cosmos EVM
    evmmodule "github.com/evmos/os/x/evm"
    evmkeeper "github.com/evmos/os/x/evm/keeper"
    evmtypes "github.com/evmos/os/x/evm/types"
    
    // feemarket module required by EVM
    feemarketmodule "github.com/evmos/os/x/feemarket"
    feemarketkeeper "github.com/evmos/os/x/feemarket/keeper"
    feemarkettypes "github.com/evmos/os/x/feemarket/types"
)

type FunaiApp struct {
    // Existing keepers ...
    SettlementKeeper  settlementkeeper.Keeper
    WorkerKeeper      workerkeeper.Keeper
    // ...
    
    // EVM additions
    EvmKeeper         evmkeeper.Keeper
    FeeMarketKeeper   feemarketkeeper.Keeper
}

func NewFunaiApp(...) *FunaiApp {
    // ...
    
    // Initialize EVM keeper
    app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
        appCodec, keys[feemarkettypes.StoreKey], authtypes.NewModuleAddress(govtypes.ModuleName),
    )
    
    app.EvmKeeper = *evmkeeper.NewKeeper(
        appCodec, keys[evmtypes.StoreKey], tkeys[evmtypes.TransientKey],
        authtypes.NewModuleAddress(govtypes.ModuleName),
        app.AccountKeeper, app.BankKeeper, app.StakingKeeper,
        app.FeeMarketKeeper, nil, // tracer
        "", // EVM chain ID (EIP-155, e.g. 9000)
    )
    
    // Register modules
    app.ModuleManager.RegisterModules(
        // Existing modules ...
        evmmodule.NewAppModule(app.EvmKeeper, app.AccountKeeper),
        feemarketmodule.NewAppModule(app.FeeMarketKeeper),
    )
    
    // Module ordering (BeginBlock / EndBlock)
    // EVM and feemarket need to be added to BeginBlockers and EndBlockers
}
```

#### 2.3 Genesis Configuration

Add default parameters for EVM and feemarket in `genesis.json`:

```json
{
  "app_state": {
    "evm": {
      "params": {
        "evm_denom": "ufai",
        "enable_create": true,
        "enable_call": true,
        "extra_eips": [],
        "chain_config": {
          "homestead_block": "0",
          "dao_fork_block": "0",
          "dao_fork_support": true,
          "eip150_block": "0",
          "eip150_hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
          "eip155_block": "0",
          "eip156_block": "0",
          "byzantium_block": "0",
          "constantinople_block": "0",
          "petersburg_block": "0",
          "istanbul_block": "0",
          "muir_glacier_block": "0",
          "berlin_block": "0",
          "london_block": "0",
          "arrow_glacier_block": "0",
          "gray_glacier_block": "0",
          "merge_netsplit_block": "0",
          "shanghai_time": "0",
          "cancun_time": "0"
        }
      },
      "accounts": []
    },
    "feemarket": {
      "params": {
        "no_base_fee": false,
        "base_fee_change_denominator": 8,
        "elasticity_multiplier": 2,
        "enable_height": "0",
        "base_fee": "1000000000",
        "min_gas_price": "0.000000000000000000",
        "min_gas_multiplier": "0.500000000000000000"
      },
      "block_gas": "0"
    }
  }
}
```

**Key parameter: `evm_denom` must be set to `ufai`** so that EVM gas is paid in FAI, unified with the native modules.

#### 2.4 Build Verification

```bash
make build
# If there are build errors, resolve them one by one
# Common issues:
#   1. Proto file conflicts → check the proto/ directory
#   2. Keeper interface mismatches → SDK version differences
#   3. Duplicate store keys → check key naming
```

### Day 2: Configure JSON-RPC + Basic Testing

#### 2.5 Enable JSON-RPC

Edit `app.toml`:

```toml
###############################################################################
###                           EVM JSON-RPC                                  ###
###############################################################################

[json-rpc]
# Enable JSON-RPC service
enable = true
# Listen address
address = "0.0.0.0:8545"
# WebSocket address
ws-address = "0.0.0.0:8546"
# API namespaces
api = "eth,txpool,personal,net,debug,web3"
# Gas cap
gas-cap = 25000000
# EVM timeout
evm-timeout = "5s"
# TxFee cap
txfee-cap = 1
# Log filter cap
filter-cap = 200
# Enable indexer
enable-indexer = true
```

#### 2.6 MetaMask Testing

```
Network Name: FunAI Testnet
RPC URL: http://34.87.21.99:8545
Chain ID: 9000 (or custom, must match the EIP-155 chain ID in app.go)
Currency Symbol: FAI
Block Explorer: http://34.87.21.99:8088

Note: Chain ID 9000 is the default for Evmos testnet.
FunAI mainnet should use its own Chain ID; verify no conflicts at https://chainlist.org.
```

#### 2.7 Deploy a Test Contract

```bash
# Deploy Hello World using cast (foundry)
cast send --rpc-url http://34.87.21.99:8545 \
  --private-key $DEPLOYER_PRIVATE_KEY \
  --create $(cat HelloWorld.bin)

# Or use Hardhat
npx hardhat run scripts/deploy.js --network funai
```

```solidity
// HelloWorld.sol — simplest test contract
pragma solidity ^0.8.0;

contract HelloWorld {
    string public message = "Hello FunAI";
    
    function setMessage(string memory _msg) public {
        message = _msg;
    }
}
```

### Day 3: Precompile Bridge (EVM <-> InferenceAccount)

#### 2.8 Why Precompiles Are Needed

EVM contracts can only operate on EVM state (contract storage) by default. To let Solidity contracts interact with FunAI native modules (InferenceAccount deposits, balance queries), bridging via precompiled contracts is required.

A precompile is a "special contract" deployed at a fixed address that actually executes Go code.

#### 2.9 Implement InferenceAccount Precompile

```go
// x/evm/precompiles/inference/inference.go

package inference

import (
    "math/big"
    
    "github.com/ethereum/go-ethereum/common"
    settlementkeeper "funai-chain/x/settlement/keeper"
)

// Fixed address: 0x0000000000000000000000000000000000000900
var PrecompileAddress = common.HexToAddress("0x0000000000000000000000000000000000000900")

type InferencePrecompile struct {
    settlementKeeper settlementkeeper.Keeper
}

// Solidity interface:
//   function deposit(address user, uint256 amount) external;
//   function balanceOf(address user) external view returns (uint256);
//   function availableBalance(address user) external view returns (uint256);

func (p *InferencePrecompile) Run(input []byte) ([]byte, error) {
    // Parse Solidity ABI-encoded call
    methodID := input[:4]
    
    switch {
    case bytes.Equal(methodID, depositMethodID):
        return p.deposit(input[4:])
    case bytes.Equal(methodID, balanceOfMethodID):
        return p.balanceOf(input[4:])
    case bytes.Equal(methodID, availableBalanceMethodID):
        return p.availableBalance(input[4:])
    }
    return nil, fmt.Errorf("unknown method")
}

func (p *InferencePrecompile) deposit(input []byte) ([]byte, error) {
    // Decode arguments
    user, amount := decodeDepositArgs(input)
    
    // Call settlement keeper's ProcessDeposit
    userAddr := sdk.AccAddress(user.Bytes())
    coin := sdk.NewCoin("ufai", sdk.NewIntFromBigInt(amount))
    
    err := p.settlementKeeper.ProcessDeposit(ctx, userAddr, coin)
    if err != nil {
        return nil, err
    }
    return successReturn(), nil
}
```

#### 2.10 Register the Precompile

Register when initializing the EVM keeper in `app.go`:

```go
precompiles := map[common.Address]vm.PrecompiledContract{
    inference.PrecompileAddress: &inference.InferencePrecompile{
        settlementKeeper: app.SettlementKeeper,
    },
}

app.EvmKeeper.WithPrecompiles(precompiles)
```

#### 2.11 Solidity Interface File

Interface for Skill developers:

```solidity
// IInferenceAccount.sol
// Deployed at address 0x0000000000000000000000000000000000000900
interface IInferenceAccount {
    // Deposit to inference account
    function deposit(address user, uint256 amount) external;
    
    // Query total balance
    function balanceOf(address user) external view returns (uint256);
    
    // Query available balance (after deducting frozen amounts)
    function availableBalance(address user) external view returns (uint256);
}
```

### Day 4-5: Testing + Security Verification

#### 2.12 Required Verification Items

| Test | What to Verify | Pass Criteria |
|------|---------|---------|
| EVM tx does not affect consensus | Send 100 EVM txs + 100 native txs | Chain produces blocks normally |
| Gas isolation | EVM tx gas and native tx gas are independent | No mutual interference |
| Precompile deposit | Solidity calls deposit -> InferenceAccount balance increases | Amounts match |
| Precompile balanceOf | Query result matches on-chain state | Exact match |
| EVM denom | EVM gas is paid in ufai | MetaMask displays FAI |
| Reentrancy attack | Solidity contract attempts to re-enter precompile | Blocked |
| Gas cap | Oversized EVM tx | Blocked by gas cap |

---

## 3. Directory Structure Changes

```
funai-chain/
├── x/
│   ├── settlement/    # Existing
│   ├── worker/        # Existing
│   ├── modelreg/      # Existing
│   ├── reward/        # Existing
│   ├── vrf/           # Existing
│   └── evm/           # New (if custom precompiles are needed)
│       └── precompiles/
│           └── inference/
│               ├── inference.go       # Precompile implementation
│               ├── inference_test.go  # Tests
│               └── abi.json          # Solidity ABI
├── app/
│   └── app.go         # Modified: register EVM + feemarket modules
├── config/
│   └── app.toml       # Modified: add [json-rpc] configuration
└── genesis.json       # Modified: add evm + feemarket default parameters
```

---

## 4. Known Risks

| Risk | Description | Mitigation |
|------|------|------|
| SDK version incompatibility | Cosmos EVM may require SDK v0.47 or v0.51 | Run go mod tidy first to confirm |
| State bloat | EVM contract storage consumes on-chain space | Set gas price to prevent spam contracts |
| Precompile reentrancy | Solidity contracts may re-enter precompiles | Add reentrancy guard in precompile implementation |
| Chain ID conflict | EIP-155 chain ID must be globally unique | Register at chainlist.org |
| JSON-RPC security | Exposing port 8545 publicly has DDoS risk | Add rate limiting + reverse proxy in production |

---

*Document version: V1*
*Date: 2026-04-03*
*Baseline: commit ce87883*
*Document suffix: KT*
