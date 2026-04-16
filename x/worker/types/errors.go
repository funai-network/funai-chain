package types

import "cosmossdk.io/errors"

var (
	ErrWorkerAlreadyRegistered = errors.Register(ModuleName, 2, "worker already registered")
	ErrWorkerNotFound          = errors.Register(ModuleName, 3, "worker not found")
	ErrInsufficientStake       = errors.Register(ModuleName, 4, "insufficient stake")
	ErrWorkerNotActive         = errors.Register(ModuleName, 5, "worker is not active")
	ErrWorkerTombstoned        = errors.Register(ModuleName, 6, "worker is tombstoned (permanently banned)")
	ErrInvalidModels           = errors.Register(ModuleName, 7, "invalid model list")
	ErrExitWaitPeriod          = errors.Register(ModuleName, 8, "exit wait period has not elapsed")
	ErrWorkerNotJailed         = errors.Register(ModuleName, 9, "worker is not jailed")
	ErrJailPeriodNotElapsed    = errors.Register(ModuleName, 10, "jail period has not elapsed")
	ErrWorkerJailed            = errors.Register(ModuleName, 11, "worker is currently jailed, cannot unstake")
)
