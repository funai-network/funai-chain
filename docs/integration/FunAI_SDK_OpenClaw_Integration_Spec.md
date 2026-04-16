# FunAI SDK and OpenClaw Integration Spec

> Date: 2026-03-25
> Source: Reverse-engineering FunAI SDK requirements from the OpenClaw end-user perspective
> Goal: When OpenClaw users select FunAI as their LLM provider, the experience should be identical to selecting OpenAI/Claude

---

## 1. Background

### 1.1 OpenClaw's Architecture

```
OpenClaw runs on the user's own device (computer/phone/VPS)
Skills are local plugins
User installs a Skill → Skill runs locally
Skill needs to call LLM → calls the LLM provider API directly from the user's device

There is no "developer server" middle layer.
User = end-user caller.
```

### 1.2 LLM Providers Currently Supported by OpenClaw

```
Anthropic (Claude)
OpenAI (GPT)
OpenRouter (aggregator)
MiniMax
Local models (Ollama)
```

User selects a provider in OpenClaw settings → enters API key → Skills automatically call that provider at runtime.

### 1.3 Goal

Add FunAI Network as a provider. User selects FunAI in settings → deposits funds → all Skills automatically perform inference through the FunAI network. The experience should be as smooth as selecting OpenAI.

---

## 2. Problem Overview

Examining FunAI from the perspective of OpenClaw end-users and Skill developers reveals the following gaps:

| # | Problem | Affected Party | Severity | Resolution Layer | Effort |
|---|------|---------|---------|--------|--------|
| 1 | model_id is a bytes32 hash, not human-readable | End users + Skill developers | P1 — cannot select a model | Protocol layer ModelReg | ~10 lines |
| 2 | No Function Calling support | Skill developers + OpenClaw framework | P1 — agent functionality is broken | SDK layer | ~200 lines |
| 3 | No Messages format support | OpenClaw framework | P1 — cannot send multi-turn conversations | SDK layer | ~100 lines |
| 4 | No standardized error codes | End users | P1 — no idea why failures happen | SDK layer | ~50 lines |
| 5 | No JSON Mode | Skill developers | P1 — structured output is unreliable | SDK layer | ~80 lines |
| 6 | No Embeddings API | End users | P2 — RAG scenarios unsupported | Protocol S7 framework already reserved | Later |
| 7 | No Guided Decoding | Skill developers | P2 — better alternative to JSON Mode | Worker side | Later |

P1 total: ~10 lines protocol layer + ~430 lines SDK layer.

---

## 3. Problem 1: Human-Readable Model Aliases (P1 — Protocol Layer)

### 3.1 Problem

```
Current: model_id = SHA256(weight_hash || quant_config_hash || runtime_image_hash)
User sees: 0xa3f7b2c9d8e1...
Skill code writes: model: "0xa3f7b2c9d8e1..."

OpenAI: model: "gpt-4o"
Venice: model: "llama-3.3-70b"
→ Human-readable, you know what it is at a glance

When users select a model in OpenClaw settings:
  OpenAI → [gpt-4o] [gpt-4o-mini] [gpt-3.5-turbo] → understandable
  FunAI → [0xa3f7...] [0xb8c2...] [0xd4e5...] → which one to pick?
```

### 3.2 Solution

Add an alias field to ModelInfo in the ModelReg module:

```go
// x/modelreg/types/model.go

type ModelInfo struct {
    // ... existing fields ...
    
    // Human-readable alias, globally unique, set at registration
    // Suggested format: {model_family}-{size}-{quant}
    // Examples: "qwen3-32b-q4", "llama3.3-70b-q4", "deepseek-coder-33b"
    Alias string `protobuf:"bytes,X,opt,name=alias" json:"alias"`
}
```

Registration rules:

```
MsgRegisterModel {
    // ... existing fields ...
    alias: string    // Required, globally unique
}

Validation:
  alias must not be empty
  alias length 3-64 characters
  alias allows only lowercase letters + digits + hyphens
  alias must be globally unique (on-chain deduplication check)
  alias is immutable after registration (to prevent confusion)
```

SDK / OpenClaw integration layer usage:

```python
# Skill code or OpenClaw configuration
model: "qwen3-32b-q4"

# SDK internal
model_id = sdk.resolve_alias("qwen3-32b-q4")  # → 0xa3f7...
# Cache the alias → model_id mapping, refresh periodically
```

OpenClaw model selection UI:

```
Select model:
  [qwen3-32b-q4]        32B general-purpose model, strong in Chinese and English
  [llama3.3-70b-q4]     70B high-quality, strongest in English
  [deepseek-coder-33b]  Code-specialized
  [qwen3-235b-moe-q4]   235B MoE, strongest in multilingual
```

Effort: ~10 lines of protocol code + SDK query caching logic.

---

## 4. Problem 2: Function Calling (P1 — SDK Layer)

### 4.1 Problem

OpenClaw is an AI agent framework. Its core capability is letting AI call tools:

```
User: "Check my schedule for tomorrow"
→ OpenClaw sends to LLM (with tools definition)
→ LLM returns: {"tool": "get_calendar", "args": {"date": "2026-03-27"}}
→ OpenClaw executes get_calendar → gets result → feeds back to LLM
→ LLM: "You have 3 meetings tomorrow..."
```

OpenAI/Claude natively support function calling / tool use. FunAI only returns plain text. OpenClaw's agent loop breaks on FunAI.

### 4.2 Solution

Implement OpenAI-compatible function calling at the SDK layer. No protocol layer changes needed.

#### 4.2.1 Input Conversion

Developers (or the OpenClaw framework) pass in tools definitions:

```python
response = funai_sdk.chat_completion(
    model="qwen3-32b-q4",
    messages=[
        {"role": "system", "content": "You are an assistant"},
        {"role": "user", "content": "Check my schedule for tomorrow"},
    ],
    tools=[
        {
            "type": "function",
            "function": {
                "name": "get_calendar",
                "description": "Query the schedule for a specified date",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "date": {"type": "string", "description": "Date, format YYYY-MM-DD"}
                    },
                    "required": ["date"]
                }
            }
        }
    ]
)
```

SDK internally injects tools into the system prompt:

```
You are an assistant.

You have the following tools available. When you need to call a tool, you must reply strictly in the following JSON format without including any other text:

{"tool_call": {"name": "tool_name", "arguments": {parameters}}}

Available tools:
1. get_calendar - Query the schedule for a specified date
   Parameters:
   - date (string, required): Date, format YYYY-MM-DD

If you do not need to call a tool, reply normally.
```

#### 4.2.2 Output Parsing

SDK parses the model output after receiving it:

```python
def parse_response(text: str) -> dict:
    # Attempt to extract tool_call JSON
    # Supports multiple formats:
    #   Pure JSON: {"tool_call": {...}}
    #   Wrapped in code block: ```json\n{"tool_call": {...}}\n```
    #   With prefix text: "Sure, let me call the tool\n{"tool_call": {...}}"
    
    tool_call = extract_json(text, key="tool_call")
    
    if tool_call:
        return {
            "type": "tool_call",
            "function": {
                "name": tool_call["name"],
                "arguments": json.dumps(tool_call["arguments"])
            }
        }
    else:
        return {
            "type": "text",
            "content": text
        }
```

#### 4.2.3 Return OpenAI-Compatible Format

```python
# Format returned by SDK to OpenClaw is fully OpenAI-compatible

# Case A: AI decides to call a tool
{
    "choices": [{
        "message": {
            "role": "assistant",
            "content": null,
            "tool_calls": [{
                "id": "call_abc123",
                "type": "function",
                "function": {
                    "name": "get_calendar",
                    "arguments": "{\"date\": \"2026-03-27\"}"
                }
            }]
        },
        "finish_reason": "tool_calls"
    }]
}

# Case B: AI replies normally
{
    "choices": [{
        "message": {
            "role": "assistant",
            "content": "You have 3 meetings tomorrow..."
        },
        "finish_reason": "stop"
    }]
}
```

#### 4.2.4 Tool Result Feedback

After OpenClaw executes the tool, it feeds the result back:

```python
messages.append({"role": "assistant", "content": null, "tool_calls": [...]})
messages.append({"role": "tool", "tool_call_id": "call_abc123", "content": "{schedule result}"})

# SDK also injects tool results into the prompt
# Model sees the tool's returned result → generates the final reply
```

#### 4.2.5 Model Compatibility

```
Open-source model support for function calling:

Qwen3 series: trained with extensive tool use data → good support
Llama 3.3: trained with function calling → good support
DeepSeek: supports function calling format
Mistral: supports function calling format

These models see standard tool definition prompts → output correct JSON format
Expected SDK parse success rate > 95%

On parse failure:
  SDK auto-retries (up to 2 times)
  On retry, appends to the end of the prompt: "Please reply strictly in JSON format"
  → Post-retry success rate > 99%
```

Effort: ~200 lines of SDK code.

---

## 5. Problem 3: Messages Format (P1 — SDK Layer)

### 5.1 Problem

OpenAI/Claude/Venice all accept a messages array:

```python
messages = [
    {"role": "system", "content": "You are..."},
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hello!"},
    {"role": "user", "content": "Help me write code"},
]
```

FunAI InferRequest only has a single prompt string. The OpenClaw framework sends a messages array directly, and something needs to convert it into a prompt.

### 5.2 Solution

SDK automatically selects the appropriate chat template based on model type to concatenate messages.

#### 5.2.1 Chat Template Registration

Each model can include chat_template information when registering in ModelReg (or the SDK has built-in templates for common models):

```python
# SDK built-in templates

TEMPLATES = {
    "chatml": {
        # Used by Qwen3, Mistral, etc.
        "system": "<|im_start|>system\n{content}<|im_end|>\n",
        "user": "<|im_start|>user\n{content}<|im_end|>\n",
        "assistant": "<|im_start|>assistant\n{content}<|im_end|>\n",
        "tool": "<|im_start|>tool\n{content}<|im_end|>\n",
        "generation_prefix": "<|im_start|>assistant\n",
    },
    "llama3": {
        # Used by Llama 3.x
        "system": "<|start_header_id|>system<|end_header_id|>\n\n{content}<|eot_id|>",
        "user": "<|start_header_id|>user<|end_header_id|>\n\n{content}<|eot_id|>",
        "assistant": "<|start_header_id|>assistant<|end_header_id|>\n\n{content}<|eot_id|>",
        "generation_prefix": "<|start_header_id|>assistant<|end_header_id|>\n\n",
    },
}

# Model alias to template mapping
MODEL_TEMPLATES = {
    "qwen3-32b-q4": "chatml",
    "qwen3-235b-moe-q4": "chatml",
    "llama3.3-70b-q4": "llama3",
    "deepseek-coder-33b": "chatml",
}
```

#### 5.2.2 Conversion Logic

```python
def messages_to_prompt(messages: list, model: str) -> str:
    template_name = MODEL_TEMPLATES.get(model, "chatml")  # Default to chatml
    template = TEMPLATES[template_name]
    
    prompt = ""
    for msg in messages:
        role = msg["role"]
        content = msg["content"] or ""
        
        if role == "system":
            prompt += template["system"].format(content=content)
        elif role == "user":
            prompt += template["user"].format(content=content)
        elif role == "assistant":
            prompt += template["assistant"].format(content=content)
        elif role == "tool":
            prompt += template["tool"].format(content=content)
    
    # Append generation prefix at the end so the model knows it's its turn to speak
    prompt += template["generation_prefix"]
    
    return prompt
```

#### 5.2.3 SDK Interface

```python
# SDK external interface is fully OpenAI-compatible

response = funai_sdk.chat_completion(
    model="qwen3-32b-q4",
    messages=[
        {"role": "system", "content": "You are an assistant"},
        {"role": "user", "content": "Hello"},
    ],
    temperature=0.7,
    max_tokens=500,
)

# SDK internal:
#   1. messages → prompt (via chat template)
#   2. prompt → InferRequest → FunAI network
#   3. Model output → wrapped in OpenAI format and returned
```

#### 5.2.4 Streaming Support

```python
# Streaming interface is also OpenAI-compatible

stream = funai_sdk.chat_completion(
    model="qwen3-32b-q4",
    messages=[...],
    stream=True,
)

for chunk in stream:
    # chunk format is identical to OpenAI streaming
    # {"choices": [{"delta": {"content": "Hi"}}]}
    print(chunk.choices[0].delta.content, end="")
```

Effort: ~100 lines of SDK code.

---

## 6. Problem 4: Standardized Error Codes (P1 — SDK Layer)

### 6.1 Problem

FunAI inference can fail for various reasons. Users see "request failed" in OpenClaw but have no idea why.

### 6.2 Solution

SDK returns standardized error codes, compatible with the OpenAI error format:

```python
class FunAIError:
    INSUFFICIENT_BALANCE = "insufficient_balance"
    MODEL_NOT_FOUND = "model_not_found"
    NO_AVAILABLE_WORKER = "no_available_worker"
    REQUEST_TIMEOUT = "request_timeout"
    FEE_TOO_LOW = "fee_too_low"
    CONTENT_TAG_MISMATCH = "content_tag_no_worker"
    MAX_TOKENS_EXCEEDED = "max_tokens_exceeded"
    INVALID_PARAMETERS = "invalid_parameters"
    NETWORK_ERROR = "network_error"
```

#### 6.2.1 Error Mapping

| Chain/P2P Layer Event | SDK Error Code | User-Facing Message | HTTP Status Code |
|----------------|-----------|--------------|------------|
| Leader checks: insufficient balance | insufficient_balance | "Insufficient FunAI balance, please top up" | 402 |
| model_id has no registered Workers | model_not_found | "No workers online for this model" | 404 |
| 3 consecutive ranks all reject | no_available_worker | "No workers available, please try again later" | 503 |
| SDK timeout (5s/10s) with no tokens received | request_timeout | "Request timed out, retrying..." | 408 |
| Fee below all Worker acceptance thresholds | fee_too_low | "Bid too low, please increase or use auto-pricing" | 422 |
| No Worker accepts this content_tag | content_tag_no_worker | "No workers currently support this content type" | 503 |
| max_tokens exceeds model limit | max_tokens_exceeded | "Output length exceeds model limit" | 400 |
| Parameter format error | invalid_parameters | "Parameter error: {details}" | 400 |
| P2P connection failure | network_error | "Network connection failed, please check your network" | 500 |

#### 6.2.2 Response Format (OpenAI-Compatible)

```python
# Error response format is identical to OpenAI
{
    "error": {
        "message": "Insufficient FunAI balance, please top up",
        "type": "insufficient_balance",
        "code": "insufficient_balance"
    }
}
```

#### 6.2.3 OpenClaw Integration Layer Handling

```python
# OpenClaw's FunAI provider module
try:
    response = funai_sdk.chat_completion(...)
except FunAIError as e:
    if e.code == "insufficient_balance":
        show_notification("Insufficient FunAI balance, please top up in settings")
    elif e.code == "request_timeout":
        # Auto-retry
        response = funai_sdk.chat_completion(...)
    elif e.code == "fee_too_low":
        # Auto-retry with higher fee
        response = funai_sdk.chat_completion(..., fee_multiplier=1.5)
    else:
        show_error(e.message)
```

Effort: ~50 lines of SDK code.

---

## 7. Problem 5: JSON Mode (P1 — SDK Layer)

### 7.1 Problem

Many OpenClaw operations depend on AI returning pure JSON. OpenAI's `response_format: {type: "json_object"}` guarantees valid JSON output. FunAI lacks this → the model may return output with prefix text or irregular formatting → OpenClaw parsing fails.

### 7.2 Solution: Two Phases

#### 7.2.1 Phase 1: SDK-Layer Constraints + Validation + Retry

```python
def chat_completion(self, messages, response_format=None, **kwargs):
    
    if response_format and response_format.get("type") == "json_object":
        # Append JSON constraint to the end of the system prompt
        json_constraint = (
            "\n\n[IMPORTANT] You must return only a valid JSON object."
            "Do not include any other text, explanations, code block markers, or prefixes."
            "Start directly with { and end with }."
        )
        messages = self._append_to_system(messages, json_constraint)
    
    # Send request
    for attempt in range(3):  # Up to 3 attempts
        raw_response = self._raw_inference(messages, **kwargs)
        text = raw_response.text
        
        if response_format and response_format.get("type") == "json_object":
            # Try to extract JSON
            json_obj = self._extract_json(text)
            if json_obj is not None:
                return self._wrap_response(json.dumps(json_obj))
            
            # Extraction failed → retry
            if attempt < 2:
                # Append stronger constraint
                messages = self._append_retry_hint(messages, text)
                continue
            else:
                raise FunAIError("json_parse_failed", "Model did not return valid JSON")
        
        return self._wrap_response(text)
```

JSON extraction logic (fault-tolerant):

```python
def _extract_json(self, text: str) -> dict | None:
    """Extract JSON from model output, tolerating common formatting issues"""
    
    # Method 1: Direct parse
    try:
        return json.loads(text.strip())
    except:
        pass
    
    # Method 2: Strip code block markers
    # ```json\n{...}\n```
    match = re.search(r'```(?:json)?\s*(\{.*?\})\s*```', text, re.DOTALL)
    if match:
        try:
            return json.loads(match.group(1))
        except:
            pass
    
    # Method 3: Find first { to last }
    start = text.find('{')
    end = text.rfind('}')
    if start != -1 and end != -1 and end > start:
        try:
            return json.loads(text[start:end+1])
        except:
            pass
    
    return None
```

#### 7.2.2 Phase 2 (Future): Worker-Side Guided Decoding

vLLM supports `--guided-decoding-backend outlines`, which constrains output tokens during generation to conform to a JSON schema.

```
Advantage: 100% guaranteed valid JSON, succeeds on first generation, no retries needed
Prerequisites: Workers enable guided decoding + protocol/SDK passes JSON schema

Implementation path:
  Add optional json_schema field to InferRequest (not signed, hint only)
  Workers configure vLLM with --guided-decoding-backend outlines
  Receive request with json_schema → constrained generation
  
  Not implementing this doesn't affect functionality (Phase 1 SDK solution already covers it)
  Implementing it improves experience (first-try success vs up to 3 retries)
```

Effort: Phase 1 ~80 lines of SDK code. Phase 2 ~30 lines (optional).

---

## 8. Native OpenClaw Integration Plan

### 8.1 Integration Architecture

```
OpenClaw on user's device
  ├── Config: LLM Provider = FunAI Network
  ├── FunAI Provider Module (new)
  │     ├── FunAI SDK (embedded)
  │     ├── Wallet management (create/import/balance query)
  │     └── OpenAI compatibility layer (messages/tools/errors conversion)
  │
  └── Skill Runtime
        → Calls LLM → OpenClaw routes to FunAI Provider
        → FunAI SDK → P2P → FunAI network → Worker inference
        → SDK returns OpenAI-compatible format → Skill processes normally
```

### 8.2 User First-Time Setup Flow

```
OpenClaw Settings → LLM Provider → Select "FunAI Network"

First selection:
  "FunAI Network — 30-50% cheaper, no account bans"
  
  [Create New Wallet]  [Import Existing Wallet]

  Create new wallet:
    → SDK generates key pair locally → displays mnemonic → user backs up
    → "Your FunAI address: funai1x7k...3f9"
    → Top up:
      [Top up with TON]  → Connect Telegram TON wallet → transfer in
      [Top up with USDC] → Display deposit address → user transfers in
    → Deposit received → Gateway auto-converts to FAI → balance displayed

  Import existing wallet:
    → Enter mnemonic or private key → import
    → Display balance

Setup complete:
  ┌─────────────────────────────┐
  │  FunAI Network              │
  │  Balance: 468 FAI (≈ $9.36) │
  │  Model: qwen3-32b-q4        │
  │  [Top Up]  [Switch Model]  [Settings] │
  └─────────────────────────────┘
```

### 8.3 Daily Usage

```
User uses any Skill → Skill calls LLM → automatically goes through FunAI

User doesn't need to do anything. Exactly the same as using OpenAI.

Only difference:
  OpenAI → balance is on the OpenAI website
  FunAI → balance is visible directly in OpenClaw settings

When balance is insufficient:
  OpenClaw shows notification: "Insufficient FunAI balance, please top up"
  → Click → Jump to top-up page
```

### 8.4 Model Selection

```
OpenClaw Settings → FunAI → Model Selection

Displays all available models (SDK queries from on-chain ModelReg):

  Recommended:
  ⭐ qwen3-32b-q4        General-purpose, strong in Chinese & English, best value
  
  All models:
  📝 qwen3-32b-q4        32B general       $0.10/$0.50 per M token
  📝 qwen3-235b-moe-q4   235B MoE multi    $0.15/$0.75 per M token
  📝 llama3.3-70b-q4     70B strongest EN   $0.40/$1.20 per M token
  💻 deepseek-coder-33b  Code-specialized   $0.10/$0.50 per M token

Each model displays:
  Alias (human-readable)
  Description
  Current online worker count
  Current average price
  
User selects one → all Skills default to this model
Skills can also specify a particular model in code (overrides default)
```

### 8.5 Auto-Pricing

```
Users don't need to manually set fees. SDK handles it automatically.

SDK auto-pricing logic:
  1. Query the average price of the last 100 transactions for this model_id on-chain
  2. fee = average price × 1.1 (add 10% to ensure order acceptance)
  3. max_fee = fee × max_tokens (upper bound protection)
  
  User only sees:
    "Estimated cost for this request: 0.5 FAI (≈ $0.01)"
    
  Actual charges are per-request (currently, per-token later)

Advanced users can manually set fees:
  OpenClaw Settings → FunAI → Advanced → Custom Bid
  → Most users won't touch this
```

---

## 9. Complete SDK Interface Design

### 9.1 Initialization

```python
import funai_sdk

# Method A: With wallet (direct chain connection, for end users)
client = funai_sdk.Client(
    wallet_path="~/.funai/wallet.json",  # Local wallet
    network="mainnet",
)

# Method B: With Gateway API key (for developers/testing)
client = funai_sdk.Client(
    base_url="https://gateway.funai.network/v1",
    api_key="funai_xxx",
)
```

### 9.2 Chat Completion (Core Interface)

```python
# Interface fully identical to OpenAI

response = client.chat.completions.create(
    model="qwen3-32b-q4",
    messages=[
        {"role": "system", "content": "You are an assistant"},
        {"role": "user", "content": "Hello"},
    ],
    temperature=0.7,
    max_tokens=500,
    tools=[...],                    # Optional, function calling
    response_format={"type": "json_object"},  # Optional, JSON mode
    stream=False,                   # Optional, streaming
    content_tag="general",          # Optional, content tag
)

# Return format is fully identical to OpenAI
print(response.choices[0].message.content)
```

### 9.3 Streaming

```python
stream = client.chat.completions.create(
    model="qwen3-32b-q4",
    messages=[...],
    stream=True,
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### 9.4 Model List

```python
models = client.models.list()

for m in models.data:
    print(f"{m.id}  {m.alias}  workers={m.active_workers}  price={m.avg_price}")
```

### 9.5 Balance Query

```python
balance = client.balance.get()
print(f"Balance: {balance.fai} FAI (≈ ${balance.usd})")
```

### 9.6 SDK Internal Flow

```
What happens inside client.chat.completions.create():

1. model alias → model_id (query on-chain ModelReg, locally cached)
2. messages → prompt (chat template conversion)
3. tools → injected into system prompt (function calling)
4. response_format → append JSON constraint to system prompt
5. Auto-pricing → query on-chain average price → set fee
6. Build InferRequest → sign → send to Leader via P2P
7. Await streaming response → collect tokens
8. If tools present → parse function call
9. If JSON mode → validate JSON → retry on failure
10. Wrap in OpenAI-compatible format → return

Timeout/errors:
  5s with no tokens → resend (same task_id)
  Insufficient balance → raise insufficient_balance
  Model doesn't exist → raise model_not_found
  → Standardized error codes
```

---

## 10. Non-Issues (Removed from the Original 10)

| Original Issue | Why It's Not a Problem |
|--------|--------------|
| Streaming resumption from breakpoint | Worker disconnection probability is low; on disconnect, SDK resends from scratch after 5s; users can tolerate occasional occurrences |
| Batch inference | OpenClaw end-users don't need it; will be built later in Gateway for API developers |
| Rate limiting | Users are using it for themselves; balance runs out and it naturally stops; no need to limit oneself |
| Context Caching | SDK prompt compression + vLLM APC already covers this; no additional protocol needed |

---

## 11. Implementation Plan

### 11.1 Dependencies

```
Model aliases (protocol layer) → SDK model resolution depends on this
Messages format → Function Calling depends on this (tools are injected into messages before converting to prompt together)
JSON Mode → Independent
Error codes → Independent
OpenClaw integration → Depends on all of the above
```

### 11.2 Development Order

```
Week 1: Basic SDK
  Model alias query + caching
  Messages → prompt conversion (chat template)
  InferRequest construction + signing + P2P sending
  Streaming reception + OpenAI format response
  Auto-pricing
  Error codes
  → SDK is functional

Week 2: Advanced Features
  Function Calling (tools injection + output parsing)
  JSON Mode (constraints + validation + retry)
  Wallet management (create/import/balance)
  
Week 3: OpenClaw Integration
  FunAI provider module
  Settings UI (model selection/top-up/balance)
  Skill runtime routing
  Test all Skill compatibility

Week 4: Testing + Launch
  Compatibility testing with 10 commonly used Skills
  Error handling edge cases
  Documentation
```

### 11.3 Code Estimates

```
Protocol layer:
  ModelReg alias                      ~10 lines

SDK layer:
  Core (request/signing/P2P/streaming) ~300 lines
  Messages format conversion           ~100 lines
  Function Calling                    ~200 lines
  JSON Mode                           ~80 lines
  Error codes                         ~50 lines
  Auto-pricing                        ~50 lines
  Wallet management                   ~100 lines
  Model list/balance query            ~50 lines
  ────────────────────────────────
  SDK total                           ~930 lines

OpenClaw integration layer:
  Provider module                     ~200 lines
  Settings UI                         ~300 lines
  ────────────────────────────────
  Integration total                   ~500 lines

Grand total                           ~1,440 lines
```

---

## 12. Relationship with AI Companion Product

```
The SDK is shared. AI Companion and OpenClaw use the same SDK.

AI Companion uses:
  chat.completions.create(messages=[...])
  → Text chat

OpenClaw Skills use:
  chat.completions.create(messages=[...], tools=[...])
  → Agent tool calling

Same SDK, same network, same pool of workers.
Two types of demand overlapping → higher worker utilization → lower costs.
AI Companion chatting during the day + OpenClaw working during the day → complementary load.
```

---

## 13. FAI Token Transition as Reflected in OpenClaw

```
Phase 1: Users don't touch FAI
  Top up with TON/USDC in OpenClaw settings → SDK backend auto-converts to FAI
  User sees: "Balance $9.36"
  FAI amount shown in small text: "≈ 468 FAI"

Phase 2: Users see FAI
  Balance display changes to FAI as primary: "468 FAI (≈ $9.36)"
  Model pricing displayed in FAI: "0.5 FAI / 1K tokens"

Phase 3: Users actively use FAI
  "Top up directly with FAI for 10% off"
  → Users buy FAI on DEX → deposit into OpenClaw wallet
  → Save money

Transition path is identical to the AI Companion's.
```

---

*Document version: V1*
*Date: 2026-03-25*
