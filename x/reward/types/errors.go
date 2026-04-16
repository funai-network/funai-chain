package types

import (
	"cosmossdk.io/errors"
)

var (
	ErrInvalidParams   = errors.Register(ModuleName, 2, "invalid params")
	ErrInvalidGenesis  = errors.Register(ModuleName, 3, "invalid genesis state")
	ErrNoContributions = errors.Register(ModuleName, 4, "no worker contributions found")
	ErrInvalidAddress  = errors.Register(ModuleName, 5, "invalid worker address")
	ErrRewardOverflow  = errors.Register(ModuleName, 7, "reward calculation overflow")
	ErrNoOnlineWorkers = errors.Register(ModuleName, 8, "no online workers available for stake distribution")
)
