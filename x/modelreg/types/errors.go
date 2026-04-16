package types

import "cosmossdk.io/errors"

var (
	ErrModelNotFound             = errors.Register(ModuleName, 2, "model not found")
	ErrModelAlreadyExists        = errors.Register(ModuleName, 3, "model already exists")
	ErrInvalidModelId            = errors.Register(ModuleName, 4, "invalid model id")
	ErrInvalidEpsilon            = errors.Register(ModuleName, 5, "invalid epsilon value")
	ErrActivationThresholdNotMet = errors.Register(ModuleName, 6, "activation threshold not met")
	ErrModelCannotServe          = errors.Register(ModuleName, 7, "model cannot serve: installed stake ratio below service threshold")
	ErrInvalidAlias              = errors.Register(ModuleName, 8, "invalid alias")
	ErrAliasAlreadyTaken         = errors.Register(ModuleName, 9, "alias already taken")
)
