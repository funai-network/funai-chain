package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	funaiapp "github.com/funai-wiki/funai-chain/app"
	funaiclient "github.com/funai-wiki/funai-chain/sdk"
)

// e2e-client sends an inference request via the FunAI SDK and prints the result.
// Used by scripts/e2e-real-inference.sh for end-to-end testing with a real TGI backend.
//
// Environment variables:
//   E2E_BOOT_PEERS    — comma-separated bootstrap peer multiaddrs
//   E2E_CHAIN_RPC     — chain RPC URL (default http://localhost:36657)
//   E2E_CHAIN_REST    — chain REST URL (default http://localhost:11317)
//   E2E_USER_PRIVKEY  — hex-encoded secp256k1 private key (32 bytes)
//   E2E_MODEL_ID      — model ID to request (default "qwen-test")
//   E2E_PROMPT        — inference prompt (default "What is 2+2?")
//   E2E_FEE           — fee in ufai (default 1000000)
//   E2E_TEMPERATURE   — temperature * 10000 (default 0 = greedy)
//   E2E_TIMEOUT       — timeout in seconds (default 120)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Initialize bech32 address prefixes for "funai1..." format
	funaiapp.SetAddressPrefixes()

	bootPeers := splitComma(getEnv("E2E_BOOT_PEERS", ""))
	chainRPC := getEnv("E2E_CHAIN_RPC", "http://localhost:36657")
	chainREST := getEnv("E2E_CHAIN_REST", "http://localhost:11317")
	privKeyHex := os.Getenv("E2E_USER_PRIVKEY")
	modelId := getEnv("E2E_MODEL_ID", "qwen-test")
	prompt := getEnv("E2E_PROMPT", "What is 2+2? Answer with just the number.")
	feeStr := getEnv("E2E_FEE", "1000000")
	tempStr := getEnv("E2E_TEMPERATURE", "0")
	timeoutStr := getEnv("E2E_TIMEOUT", "120")

	fee, _ := strconv.ParseUint(feeStr, 10, 64)
	temp, _ := strconv.ParseUint(tempStr, 10, 16)
	timeout, _ := strconv.Atoi(timeoutStr)

	if privKeyHex == "" {
		log.Fatal("E2E_USER_PRIVKEY is required")
	}

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		log.Fatalf("Invalid E2E_USER_PRIVKEY: %v", err)
	}
	privKey := secp256k1.PrivKey(privKeyBytes)
	pubKey := privKey.PubKey()
	bech32Addr := sdk.AccAddress(pubKey.Address()).String()

	// Derive-only mode: just print the bech32 address and exit
	if os.Getenv("E2E_DERIVE_ONLY") == "1" {
		fmt.Printf("ADDRESS:%s\n", bech32Addr)
		os.Exit(0)
	}

	fmt.Println("=== FunAI E2E Inference Client ===")
	fmt.Printf("Model:       %s\n", modelId)
	fmt.Printf("Prompt:      %s\n", prompt)
	fmt.Printf("Fee:         %d ufai\n", fee)
	fmt.Printf("Temperature: %d (raw) / %.2f (actual)\n", temp, float64(temp)/10000.0)
	fmt.Printf("Boot peers:  %d\n", len(bootPeers))
	fmt.Printf("Chain RPC:   %s\n", chainRPC)
	fmt.Printf("User pubkey: %x\n", pubKey.Bytes())
	fmt.Printf("User addr:   %s\n", bech32Addr)
	fmt.Println()

	cfg := funaiclient.Config{
		ListenAddr:          "/ip4/0.0.0.0/tcp/0",
		BootPeers:           bootPeers,
		ChainRPC:            chainRPC,
		ChainREST:           chainREST,
		UserPubkey:          pubKey.Bytes(),
		UserPrivKey:         privKeyBytes,
		DisableSanitization: true,
		PrivacyMode:         "plain",
	}

	client, err := funaiclient.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create SDK client: %v", err)
	}
	defer client.Close()

	// Give P2P time to discover peers
	fmt.Println("Waiting 5s for P2P peer discovery...")
	time.Sleep(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	fmt.Println("Sending inference request...")
	start := time.Now()

	result, err := client.Infer(ctx, funaiclient.InferParams{
		ModelId:     modelId,
		Prompt:      prompt,
		Fee:         fee,
		Temperature: uint16(temp),
		MaxTokens:   256,
	})
	if err != nil {
		log.Fatalf("Inference failed: %v", err)
	}

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println("=== Inference Result ===")
	fmt.Printf("Task ID:     %x\n", result.TaskId[:16])
	fmt.Printf("Output:      %s\n", result.Output)
	fmt.Printf("Tokens:      %d\n", len(result.Tokens))
	fmt.Printf("Verified:    %v\n", result.Verified)
	fmt.Printf("Result hash: %x\n", result.ResultHash[:16])
	fmt.Printf("Latency:     %s\n", elapsed)
	fmt.Println()

	if result.Output == "" {
		fmt.Println("FAIL: Empty output")
		os.Exit(1)
	}

	fmt.Println("SUCCESS: Inference completed successfully")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
