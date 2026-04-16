# EVM Integration

FunAI Chain integrates EVM compatibility via [Cosmos EVM](https://github.com/evmos/os) (formerly evmOS), maintained by Interchain Labs under the Apache 2.0 license. This allows Solidity smart contracts to interact with native chain modules, including the [settlement keeper](settlement.md) through a precompile bridge.

Source: [CosmosEVM Integration KT](../docs/FunAI_CosmosEVM_Integration_KT.md)

---

## Dependency

Use **`github.com/evmos/os`** (Cosmos EVM). Do NOT use:

- `evmos/ethermint` -- deprecated, no longer maintained.
- `cosmos/ethermint` -- older fork, incompatible with current Cosmos SDK.

---

## Chain Parameters

| Parameter | Value |
|-----------|-------|
| EVM denom | `ufai` (unified gas payment with native token) |
| EIP-155 Chain ID | `333` |
| Ethereum forks enabled | All forks from block 0 (Homestead through Cancun) |

All Ethereum hard forks are activated at genesis (block 0), so the EVM behaves identically to post-Cancun mainnet Ethereum from the start.

---

## JSON-RPC Configuration

| Setting | Value |
|---------|-------|
| HTTP port | `8545` |
| WebSocket port | `8546` |
| API namespaces | `eth`, `txpool`, `personal`, `net`, `debug`, `web3` |
| Gas cap | 25,000,000 |
| EVM timeout | 5 seconds |

---

## Precompile Bridge

A stateful precompile is deployed at a fixed address to bridge EVM contracts with on-chain [settlement](settlement.md) state:

| Field | Value |
|-------|-------|
| Address | `0x0000000000000000000000000000000000000900` |
| Routes to | Settlement keeper |

### Methods

| Signature | Description |
|-----------|-------------|
| `deposit(address, uint256)` | Deposit funds into the inference balance for a given address |
| `balanceOf(address)` | Query the total on-chain balance for an address |
| `availableBalance(address)` | Query available balance (total minus pending settlements) |

These methods call directly into the settlement keeper, so balance changes from precompile calls are reflected immediately in on-chain state used by [overspend protection](overspend-protection.md) and [per-token billing](per-token-billing.md).

---

## Known Risks

| Risk | Description |
|------|-------------|
| SDK version incompatibility | Cosmos SDK upgrades may break Cosmos EVM integration; pin and test versions carefully |
| State bloat | EVM contract storage grows unbounded; monitor state size and consider pruning strategies |
| Precompile reentrancy | Malicious contracts could attempt reentrant calls to the precompile bridge; guard settlement keeper calls |
| Chain ID collision | EIP-155 Chain ID `333` must not conflict with other EVM networks the ecosystem interacts with |
| JSON-RPC DDoS | Public JSON-RPC endpoints are vulnerable to denial-of-service; rate-limit and firewall in production |
