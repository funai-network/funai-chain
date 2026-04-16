#!/usr/bin/env python3
"""
Mock TGI server for E2E testing without a real GPU.
Responds to /generate, /generate_stream, /tokenize endpoints
with deterministic outputs that mimic HuggingFace TGI responses.

Usage:
  python3 scripts/mock-tgi.py [port]                    # default port 8080, TGI v2 mode
  python3 scripts/mock-tgi.py [port] --tgi-version 3    # TGI v3 mode (empty prefill, wrapped tokenize)
"""

import json
import sys
import hashlib
from http.server import HTTPServer, BaseHTTPRequestHandler

# Global TGI version mode (set from CLI args)
TGI_VERSION = 2


class MockTGIHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        try:
            data = json.loads(body) if body else {}
        except json.JSONDecodeError:
            data = {}

        if self.path == "/generate":
            self.handle_generate(data)
        elif self.path == "/generate_stream":
            self.handle_generate_stream(data)
        elif self.path == "/tokenize":
            self.handle_tokenize(data)
        else:
            self.send_error(404)

    def do_GET(self):
        if self.path == "/health":
            self.send_json({"status": "ok"})
        elif self.path == "/info":
            info = {
                "model_id": "mock-qwen-0.5b",
                "model_dtype": "float16",
                "max_concurrent_requests": 128,
                "max_input_length": 4096,
                "max_total_tokens": 8192,
            }
            if TGI_VERSION >= 3:
                info["version"] = "3.3.6-dev0"
                info["max_input_tokens"] = info.pop("max_input_length")
            else:
                info["version"] = "2.0.4"
            self.send_json(info)
        else:
            self.send_error(404)

    def handle_generate(self, data):
        prompt = data.get("inputs", "")
        params = data.get("parameters", {})
        max_tokens = params.get("max_new_tokens", 50)
        temperature = params.get("temperature", 0)
        details = params.get("details", False)
        decoder_input_details = params.get("decoder_input_details", False)
        top_n_tokens = params.get("top_n_tokens", 0)

        # Generate deterministic output based on prompt hash
        output_tokens = self.generate_tokens(prompt, max_tokens, temperature)
        generated_text = "".join(t["text"] for t in output_tokens)

        response = {"generated_text": generated_text}

        if details:
            logprob_key = "log_prob" if TGI_VERSION >= 3 else "logprob"
            token_infos = []
            for i, tok in enumerate(output_tokens):
                info = {
                    "id": tok["id"],
                    "text": tok["text"],
                    logprob_key: -0.5 - (i * 0.01),
                    "special": False,
                }
                if top_n_tokens > 0:
                    info["top_tokens"] = self.make_top_tokens(tok, top_n_tokens, i, logprob_key)
                token_infos.append(info)

            prefill_tokens = []
            if decoder_input_details and TGI_VERSION < 3:
                # v2: return logprobs for all input tokens in prefill
                input_words = prompt.split()
                full_words = input_words + [t["text"] for t in output_tokens]
                for j, word in enumerate(full_words):
                    prefill_tok = {
                        "id": hash(word) % 50000,
                        "text": word if j == 0 else (" " + word),
                        "logprob": -0.3 - (j * 0.005),
                        "special": False,
                    }
                    if top_n_tokens > 0:
                        prefill_tok["top_tokens"] = self.make_top_tokens(
                            {"id": prefill_tok["id"], "text": prefill_tok["text"]},
                            top_n_tokens, j, "logprob"
                        )
                    prefill_tokens.append(prefill_tok)
            # v3: decoder_input_details accepted but prefill stays empty

            response["details"] = {
                "finish_reason": "length" if len(output_tokens) >= max_tokens else "eos_token",
                "tokens": token_infos,
                "prefill": prefill_tokens,
            }

        self.send_json(response)

    def handle_generate_stream(self, data):
        prompt = data.get("inputs", "")
        params = data.get("parameters", {})
        max_tokens = params.get("max_new_tokens", 50)
        temperature = params.get("temperature", 0)

        output_tokens = self.generate_tokens(prompt, max_tokens, temperature)
        generated_text = "".join(t["text"] for t in output_tokens)

        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()

        for i, tok in enumerate(output_tokens):
            is_last = i == len(output_tokens) - 1
            event = {
                "token": {
                    "id": tok["id"],
                    "text": tok["text"],
                    "logprob": -0.5 - (i * 0.01),
                    "special": False,
                },
            }
            if is_last:
                event["generated_text"] = generated_text
                event["details"] = {
                    "finish_reason": "length",
                    "tokens": [],
                    "prefill": [],
                }

            line = f"data:{json.dumps(event)}\n\n"
            self.wfile.write(line.encode())
            self.wfile.flush()

    def handle_tokenize(self, data):
        text = data.get("inputs", "")
        # Simple whitespace-based tokenization (mock)
        words = text.split()
        tokens = []
        for i, word in enumerate(words):
            tokens.append({
                "id": hash(word) % 50000,
                "text": word if i == 0 else (" " + word),
                "start": 0,
                "stop": len(word),
            })
        # v3: wrap in object; v2: bare array
        if TGI_VERSION >= 3:
            self.send_json({"tokens": tokens})
        else:
            self.send_json(tokens)

    def generate_tokens(self, prompt, max_tokens, temperature):
        """Generate deterministic tokens based on prompt hash."""
        # Seed from prompt for reproducibility
        seed = hashlib.sha256(prompt.encode()).digest()

        # Pre-defined response fragments for common prompts
        prompt_lower = prompt.lower()
        if "2+2" in prompt_lower or "2 + 2" in prompt_lower:
            words = ["The", " answer", " is", " 4", "."]
        elif "hello" in prompt_lower or "hi" in prompt_lower:
            words = ["Hello", "!", " How", " can", " I", " help", " you", " today", "?"]
        elif "what" in prompt_lower and "name" in prompt_lower:
            words = ["I", " am", " a", " helpful", " AI", " assistant", "."]
        else:
            words = [" I", " understand", " your", " question", ".", " The", " answer",
                     " depends", " on", " the", " context", "."]

        tokens = []
        for i, word in enumerate(words[:max_tokens]):
            token_id = (int.from_bytes(seed[i % 32:(i % 32) + 2], "big") + hash(word)) % 50000
            tokens.append({"id": abs(token_id), "text": word})

        return tokens

    def make_top_tokens(self, tok, n, position, logprob_key="logprob"):
        """Generate mock top-N token alternatives."""
        top = [{"id": tok["id"], "text": tok["text"], logprob_key: -0.5 - (position * 0.01)}]
        for k in range(1, min(n, 10)):
            alt_id = (tok["id"] + k * 7) % 50000
            top.append({
                "id": alt_id,
                "text": f"<alt{k}>",
                logprob_key: -1.0 - (k * 0.3) - (position * 0.01),
            })
        return top

    def send_json(self, data):
        body = json.dumps(data).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        sys.stderr.write(f"[MockTGI] {format % args}\n")


def main():
    global TGI_VERSION
    port = 8080
    args = sys.argv[1:]
    skip_next = False
    for i, arg in enumerate(args):
        if skip_next:
            skip_next = False
            continue
        if arg == "--tgi-version":
            if i + 1 < len(args):
                TGI_VERSION = int(args[i + 1])
                skip_next = True
        elif not arg.startswith("-"):
            try:
                port = int(arg)
            except ValueError:
                pass

    server = HTTPServer(("0.0.0.0", port), MockTGIHandler)
    print(f"Mock TGI server listening on port {port} (TGI v{TGI_VERSION} mode)")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == "__main__":
    main()
