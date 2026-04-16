package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Bech32PrefixAccAddr is the bech32 prefix for account addresses.
	Bech32PrefixAccAddr = "funai"
	// Bech32PrefixAccPub is the bech32 prefix for account public keys.
	Bech32PrefixAccPub = "funaipub"
	// Bech32PrefixValAddr is the bech32 prefix for validator addresses.
	Bech32PrefixValAddr = "funaivaloper"
	// Bech32PrefixValPub is the bech32 prefix for validator public keys.
	Bech32PrefixValPub = "funaivaloperpub"
	// Bech32PrefixConsAddr is the bech32 prefix for consensus node addresses.
	Bech32PrefixConsAddr = "funaivalcons"
	// Bech32PrefixConsPub is the bech32 prefix for consensus node public keys.
	Bech32PrefixConsPub = "funaivalconspub"

	// BondDenom is the staking/bond denomination for the chain.
	BondDenom = "ufai"

	// DisplayDenom is the display denomination.
	DisplayDenom = "fai"
)

func SetAddressPrefixes() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
	config.Seal()
}
