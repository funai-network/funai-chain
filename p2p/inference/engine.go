package inference

import "context"

// Engine is the unified interface for all inference backends (TGI, vLLM, SGLang, Ollama).
// Worker, Verifier, and other components depend on this interface, not concrete implementations.
type Engine interface {
	// Complete runs inference and returns the full result with logits.
	Complete(ctx context.Context, prompt string, maxTokens int, temperature float32, topP float32, seed *int64) (*InferenceResult, error)

	// Stream runs inference and returns tokens one by one via channel.
	Stream(ctx context.Context, prompt string, maxTokens int, temperature float32, topP float32, seed *int64) (<-chan StreamToken, <-chan error)

	// TeacherForce runs teacher forcing: given prompt + complete output,
	// returns logprobs at all output positions. Used by verifiers.
	TeacherForce(ctx context.Context, prompt string, completeOutput string, outputTokenCount int) (*InferenceResult, error)

	// Tokenize returns the token list for a given text. Callers use len() for count.
	Tokenize(ctx context.Context, text string) ([]TokenizeToken, error)

	// DeterministicGenerate runs inference step-by-step with ChaCha20 sampling.
	DeterministicGenerate(ctx context.Context, prompt string, maxTokens int, temperature float32, finalSeed []byte) (*InferenceResult, error)

	// DeterministicGenerateWithBudget is like DeterministicGenerate but stops when budget exhausted.
	DeterministicGenerateWithBudget(ctx context.Context, prompt string, maxTokens int, temperature float32, finalSeed []byte, shouldStop func(outputTokens uint32) bool) (*InferenceResult, error)

	// IsHealthy checks if the backend is responsive and ready to accept requests.
	IsHealthy(ctx context.Context) bool

	// DetectVersion probes the backend to determine its version and capabilities.
	DetectVersion()

	// BackendName returns the backend type name (e.g., "tgi", "openai", "ollama").
	BackendName() string
}
