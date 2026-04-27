#!/usr/bin/env python3
"""TPS Layer 1 benchmark — single-GPU TGI throughput at varying concurrency.

Implements layer 1 of the 5-layer TPS test in
`docs/testing/FunAI_TPS_Logits_Test_Plan_KT.md`:

    Layer 1 — Single-GPU tok/s at 1/2/4/8-way concurrency

For each concurrency level it sends N parallel `/generate` requests with the
same parameters and reports per-stream tok/s, aggregate tok/s, and wall-clock.
The aggregate tok/s curve is the input the other 4 layers compose against
("total network TPS = min(layer 1 × N_GPUs, layer 2 pipeline⁻¹, layer 3
Leader, layer 4 gossipsub, layer 5 BatchSettlement)").

Stdlib only — no `aiohttp`/`requests` dependency. Runs anywhere with Python 3.

Usage:
    python3 scripts/tps_layer1_bench.py \\
        --endpoint https://<pod>-80.proxy.runpod.net \\
        --concurrency 1,2,4,8 \\
        --max-tokens 200 \\
        --json-out layer1.json
"""

import argparse
import json
import statistics
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor


def one_request(endpoint, prompt, max_new_tokens, timeout):
    """Issue one `/generate` POST and return (wall_seconds, generated_tokens).

    `temperature=0 + do_sample=False` for deterministic output. `details=true`
    so the response carries `details.tokens[]` we count to derive tok/s.
    """
    body = json.dumps(
        {
            "inputs": prompt,
            "parameters": {
                "max_new_tokens": max_new_tokens,
                "temperature": 0,
                "do_sample": False,
                "details": True,
            },
        }
    ).encode("utf-8")

    req = urllib.request.Request(
        f"{endpoint}/generate",
        data=body,
        headers={"Content-Type": "application/json"},
    )

    t_start = time.monotonic()
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        data = json.loads(resp.read())
    t_end = time.monotonic()

    wall_s = t_end - t_start
    generated = len(data.get("details", {}).get("tokens", []))
    return wall_s, generated


def measure(endpoint, n_concurrent, prompt, max_new_tokens, warmup, timeout):
    """Run one concurrency level. Includes `warmup` sequential requests
    so the first measured wave is not dominated by TGI cold-cache effects."""
    for _ in range(warmup):
        one_request(endpoint, prompt, max_new_tokens, timeout)

    wall_start = time.monotonic()
    with ThreadPoolExecutor(max_workers=n_concurrent) as ex:
        futures = [
            ex.submit(one_request, endpoint, prompt, max_new_tokens, timeout)
            for _ in range(n_concurrent)
        ]
        results = [f.result() for f in futures]
    wall_end = time.monotonic()

    per_stream_secs = [r[0] for r in results]
    total_tokens = sum(r[1] for r in results)
    wall_secs = wall_end - wall_start

    return {
        "concurrency": n_concurrent,
        "wall_seconds": round(wall_secs, 4),
        "total_tokens_generated": total_tokens,
        "aggregate_tok_per_sec": round(total_tokens / wall_secs, 2)
        if wall_secs > 0
        else 0.0,
        "per_stream_seconds": {
            "mean": round(statistics.mean(per_stream_secs), 4),
            "median": round(statistics.median(per_stream_secs), 4),
            "max": round(max(per_stream_secs), 4),
            "min": round(min(per_stream_secs), 4),
        },
        "per_stream_tok_per_sec_mean": round(
            statistics.mean(r[1] / r[0] for r in results if r[0] > 0), 2
        ),
    }


def main():
    parser = argparse.ArgumentParser(
        description="TPS Layer 1: single-GPU TGI throughput at varying concurrency",
    )
    parser.add_argument(
        "--endpoint",
        required=True,
        help="TGI endpoint base URL, e.g. https://<pod>-80.proxy.runpod.net",
    )
    parser.add_argument(
        "--concurrency",
        default="1,2,4,8",
        help="Comma-separated concurrency levels to test (default: 1,2,4,8)",
    )
    parser.add_argument(
        "--prompt",
        default="Write a short paragraph about the colour of the sky at dusk.",
        help="Inference prompt (kept short so output length, not prompt, dominates)",
    )
    parser.add_argument(
        "--max-tokens",
        type=int,
        default=200,
        help="Output token budget per request (default 200)",
    )
    parser.add_argument(
        "--warmup",
        type=int,
        default=1,
        help="Sequential warmup requests per concurrency level (default 1)",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=120,
        help="HTTP timeout per request, seconds (default 120)",
    )
    parser.add_argument(
        "--json-out",
        default=None,
        help="If given, also write the result list as JSON to this path",
    )
    args = parser.parse_args()

    levels = [int(c.strip()) for c in args.concurrency.split(",") if c.strip()]

    print("# TPS Layer 1 — single-GPU TGI throughput benchmark")
    print(f"# Endpoint:        {args.endpoint}")
    print(f"# Prompt:          {args.prompt!r}")
    print(f"# Max new tokens:  {args.max_tokens}")
    print(f"# Warmup per lvl:  {args.warmup}")
    print(f"# Timeout (s):     {args.timeout}")
    print()
    print(
        f"{'concurrency':>11} {'wall_s':>8} {'total_tok':>10} "
        f"{'agg_tok/s':>10} {'per-stream_tok/s':>18}"
    )
    print(f"{'-' * 62}")

    results = []
    for n in levels:
        r = measure(
            args.endpoint, n, args.prompt, args.max_tokens, args.warmup, args.timeout
        )
        print(
            f"{n:>11} {r['wall_seconds']:>8.2f} {r['total_tokens_generated']:>10} "
            f"{r['aggregate_tok_per_sec']:>10.2f} "
            f"{r['per_stream_tok_per_sec_mean']:>18.2f}"
        )
        results.append(r)

    if args.json_out:
        with open(args.json_out, "w") as f:
            json.dump(results, f, indent=2)
        print(f"\n# JSON written to {args.json_out}")


if __name__ == "__main__":
    main()
