package types

const (
	EventModelProposed       = "model_proposed"
	EventModelActivated      = "model_activated"
	EventModelServicePaused  = "model_service_paused"
	EventModelServiceResumed = "model_service_resumed"
	EventModelInstalled      = "model_installed"

	AttributeKeyModelId       = "model_id"
	AttributeKeyModelName     = "model_name"
	AttributeKeyProposer      = "proposer"
	AttributeKeyEpsilon       = "epsilon"
	AttributeKeyStatus        = "status"
	AttributeKeyStakeRatio    = "installed_stake_ratio"
	AttributeKeyWorkerCount   = "worker_count"
	AttributeKeyOperatorCount = "operator_count"
	AttributeKeyWorker        = "worker"
)
