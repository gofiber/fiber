#!/usr/bin/env python3
"""Prepare and measure independently resolved Fiber versions."""
import argparse
import datetime as dt
import hashlib
import itertools
import json
import math
import os
import platform
from pathlib import Path
import random
import re
import shutil
import statistics
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
SCHEMA = 1


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def atomic_json(path, value):
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, indent=2) + "\n")
    temp.replace(path)


def json_stream(text):
    decoder = json.JSONDecoder()
    rows = []
    while text.strip():
        text = text.lstrip()
        value, end = decoder.raw_decode(text)
        rows.append(value)
        text = text[end:]
    return rows


def environment(toolchain):
    env = os.environ.copy()
    for key in ("GOFLAGS", "GOEXPERIMENT", "GODEBUG", "GOGC", "GOMEMLIMIT"):
        env.pop(key, None)
    env.update(GOENV="off", GOWORK="off", GOTOOLCHAIN=toolchain, GOMAXPROCS="1")
    return env


def command(args, cwd, env, log, timeout=900):
    proc = subprocess.run(args, cwd=cwd, env=env, capture_output=True,
                          text=True, timeout=timeout)
    log.write_text(proc.stdout + proc.stderr)
    if proc.returncode:
        raise RuntimeError(f"Command failed ({proc.returncode}); inspect {log}")
    return proc.stdout


def render(template, variant):
    major = variant["major"]
    if major not in (2, 3):
        raise ValueError("Only the configured Fiber major versions are supported")
    replacements = {
        "__FIBER__": variant["module"],
        "__CTX__": "*fiber.Ctx" if major == 2 else "fiber.Ctx",
        "__BODY_BIND__": "c.BodyParser(&p)" if major == 2 else "c.Bind().Body(&p)",
        "__QUERY_BIND__": "c.QueryParser(&p)" if major == 2 else "c.Bind().Query(&p)",
        "__ORIGINS__": '"https://bench.invalid"' if major == 2 else '[]string{"https://bench.invalid"}',
        "__METHODS_HEADER__": '"GET,POST"' if major == 2 else '"GET, POST"',
        "__METHODS__": '"GET,POST"' if major == 2 else '[]string{"GET", "POST"}',
        "__NOT_FOUND__": '"Cannot GET /missing"' if major == 2 else '"Not Found"',
    }
    for before, after in replacements.items():
        template = template.replace(before, after)
    if re.search(r"__[A-Z_]+__", template):
        raise ValueError("Unresolved template token")
    return template


def parse_benchmarks(text):
    rows = []
    seen = set()
    for line in text.splitlines():
        if not line.startswith("BenchmarkRequest/"):
            continue
        fields = line.split()
        if len(fields) != 8 or fields[3] != "ns/op" or fields[5] != "B/op" or fields[7] != "allocs/op":
            raise ValueError(f"Unexpected benchmark output: {line}")
        case = re.sub(r"-\d+$", "", fields[0].split("/", 1)[1])
        if case in seen:
            raise ValueError(f"Duplicate benchmark case: {case}")
        seen.add(case)
        row = {"case": case, "iterations": int(fields[1]), "ns_per_op": float(fields[2]),
               "bytes_per_op": float(fields[4]), "allocs_per_op": float(fields[6])}
        if (row["iterations"] <= 0 or row["ns_per_op"] <= 0
                or any(not math.isfinite(row[key]) or row[key] < 0
                       for key in ("ns_per_op", "bytes_per_op", "allocs_per_op"))):
            raise ValueError(f"Invalid benchmark observation: {line}")
        rows.append(row)
    return rows


def balanced_orders(names, rounds, seed):
    if rounds <= 0:
        raise ValueError("Rounds must be positive")
    permutations = list(itertools.permutations(names))
    if rounds % len(permutations):
        raise ValueError(f"Rounds must be a multiple of {len(permutations)}")
    rng = random.Random(seed)
    orders = []
    for _ in range(rounds // len(permutations)):
        block = permutations.copy()
        rng.shuffle(block)
        orders.extend(block)
    return orders


def prepare(output):
    config = json.loads((HERE / "versions.json").read_text())
    template = (HERE / "benchmark_test.go.in").read_text()
    output.mkdir(parents=True, exist_ok=False)
    env = environment(config["toolchain"])
    go = shutil.which("go")
    if not go:
        raise RuntimeError("Go must be available on PATH")
    manifest = {
        "schema": SCHEMA, "phase": "preparing", "created_at": now(), "config": config,
        "config_sha256": digest(HERE / "versions.json"),
        "prepare_runner_sha256": digest(Path(__file__)),
        "template_sha256": digest(HERE / "benchmark_test.go.in"),
        "variants": [], "runs": [],
        "scope": "pooled request/response/user-value reset and request preparation plus in-process handler execution; no sockets, kernel/network, or app construction",
    }
    atomic_json(output / "manifest.json", manifest)
    try:
        version = command([go, "version"], output, env, output / "go-version.log").strip()
        if config["toolchain"] not in version:
            raise RuntimeError(f"Requested {config['toolchain']}, got {version}")
        settings = command([go, "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOAMD64",
                            "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT"], output, env, output / "go-env.log")
        manifest["go_version"] = version
        manifest["go_environment"] = json.loads(settings)
        manifest["runtime_environment"] = {
            "GOMAXPROCS": "1", "GODEBUG": "unset", "GOGC": "unset",
            "GOMEMLIMIT": "unset", "GOFLAGS": "unset", "GOEXPERIMENT": "unset",
        }
        for variant in config["variants"]:
            folder = output / "modules" / variant["name"]
            folder.mkdir(parents=True)
            (folder / "go.mod").write_text(
                f"module example.com/fiber-version-benchmark\n\ngo {config['toolchain'][2:]}\n\n"
                f"require {variant['module']} {variant['version']}\n")
            (folder / "benchmark_test.go").write_text(render(template, variant))
            command([go, "mod", "tidy"], folder, env, folder / "tidy.log")
            command([go, "fmt", "./..."], folder, env, folder / "format.log")
            command([go, "mod", "verify"], folder, env, folder / "verify-modules.log")
            command([go, "test", "-race", "-count=1", "-shuffle=on", "-v", "./..."],
                    folder, env, folder / "functional.log")
            modules = json_stream(command([go, "list", "-m", "-json", "all"],
                                          folder, env, folder / "modules-full.log"))
            fiber = next(row for row in modules if row["Path"] == variant["module"])
            if fiber.get("Version") != variant["version"] or fiber.get("Replace"):
                raise RuntimeError(f"Unexpected Fiber resolution: {fiber}")
            sanitized = [{key: row[key] for key in ("Path", "Version", "GoVersion", "Sum", "GoModSum")
                          if key in row} for row in modules]
            atomic_json(folder / "modules.json", sanitized)
            binary = folder / ("benchmark.exe" if os.name == "nt" else "benchmark.test")
            command([go, "test", "-c", "-o", str(binary)], folder, env, folder / "build.log")
            probe = command([str(binary), "-test.run=^$", "-test.bench=^BenchmarkRequest$",
                             "-test.benchtime=1x", "-test.count=1"], folder, env, folder / "cases.log")
            cases = sorted(row["case"] for row in parse_benchmarks(probe))
            if len(cases) != config["expected_cases"]:
                raise RuntimeError(f"Expected {config['expected_cases']} cases, got {len(cases)}")
            if manifest["variants"] and cases != manifest["variants"][0]["cases"]:
                raise RuntimeError("Benchmark cases differ between variants")
            manifest["variants"].append({
                **variant, "binary": str(binary.relative_to(output)),
                "binary_sha256": digest(binary), "go_mod_sha256": digest(folder / "go.mod"),
                "go_sum_sha256": digest(folder / "go.sum"),
                "source_sha256": digest(folder / "benchmark_test.go"),
                "cases": cases, "modules": sanitized,
            })
            atomic_json(output / "manifest.json", manifest)
            print(f"Prepared {variant['name']}: functional tests and {len(cases)} cases pass", flush=True)
        manifest["phase"] = "ready"
        manifest["prepared_at"] = now()
    except Exception as exc:
        manifest["phase"] = "prepare_failed"
        manifest["error"] = str(exc)
        raise
    finally:
        atomic_json(output / "manifest.json", manifest)


def sample(output, rounds, benchtime, cpu, seed):
    manifest = json.loads((output / "manifest.json").read_text())
    if manifest["phase"] != "ready":
        raise ValueError("Sampling requires a ready, unsampled output directory")
    if not re.fullmatch(r"[1-9]\d*(?:ms|s|x)", benchtime):
        raise ValueError("Use a positive benchtime such as 500ms, 1s, or 100000x")
    variants = {v["name"]: v for v in manifest["variants"]}
    orders = balanced_orders(list(variants), rounds, seed)
    prefix = []
    if cpu is not None:
        if not hasattr(os, "sched_getaffinity") or cpu not in os.sched_getaffinity(0):
            raise ValueError("Requested CPU affinity is not available on this host")
        taskset = shutil.which("taskset")
        if not taskset:
            raise RuntimeError("taskset is required when --cpu is provided")
        prefix = [taskset, "-c", str(cpu)]
    for variant in variants.values():
        if digest(output / variant["binary"]) != variant["binary_sha256"]:
            raise ValueError(f"Benchmark binary changed: {variant['name']}")
    lock = output / ".sampling-lock"
    lock.mkdir()
    raw = output / "raw"
    env = environment(manifest["config"]["toolchain"])
    observations = []
    try:
        raw.mkdir()
        atomic_json(lock / "owner.json", {"pid": os.getpid(), "started_at": now()})
        manifest.update(phase="sampling", rounds=rounds, benchtime=benchtime,
                        cpu_affinity=None if cpu is None else [cpu], seed=seed,
                        orders=[list(order) for order in orders], sampling_started_at=now(),
                        sample_runner_sha256=digest(Path(__file__)),
                        host={"system": platform.system(), "release": platform.release(),
                              "machine": platform.machine(), "logical_cpus": os.cpu_count(),
                              "available_cpus": sorted(os.sched_getaffinity(0))
                              if hasattr(os, "sched_getaffinity") else None})
        atomic_json(output / "manifest.json", manifest)
        for index, order in enumerate(orders):
            for name in order:
                variant = variants[name]
                binary = output / variant["binary"]
                filename = f"{index:02d}-{name}.txt"
                before = os.getloadavg() if hasattr(os, "getloadavg") else None
                started = time.monotonic()
                text = command(prefix + [str(binary), "-test.run=^$", "-test.bench=^BenchmarkRequest$",
                                         f"-test.benchtime={benchtime}", "-test.count=1"],
                               binary.parent, env, raw / filename, timeout=900)
                rows = parse_benchmarks(text)
                if sorted(row["case"] for row in rows) != variant["cases"]:
                    raise ValueError("A measurement run did not contain every validated case")
                observations.extend({**row, "variant": name, "sample": index} for row in rows)
                manifest["runs"].append({
                    "variant": name, "sample": index, "raw": "raw/" + filename,
                    "raw_sha256": digest(raw / filename), "elapsed_seconds": time.monotonic() - started,
                    "loadavg_before": before,
                    "loadavg_after": os.getloadavg() if hasattr(os, "getloadavg") else None,
                })
                atomic_json(output / "observations.json", observations)
                atomic_json(output / "manifest.json", manifest)
                print(f"{index+1}/{rounds} {name}: {len(rows)} benchmark observations", flush=True)
        manifest["phase"] = "complete"
        manifest["completed_at"] = now()
    except Exception as exc:
        manifest["phase"] = "sample_failed"
        manifest["error"] = str(exc)
        raise
    finally:
        atomic_json(output / "manifest.json", manifest)
        shutil.rmtree(lock)


def summarize(output):
    manifest = json.loads((output / "manifest.json").read_text())
    if manifest["phase"] != "complete":
        raise ValueError("Incomplete measurements cannot be summarized")
    # Raw Go output is authoritative. Check its hashes, schedule and complete
    # case set before producing any derived report.
    variants = {v["name"]: v for v in manifest["variants"]}
    expected_runs = {(name, index) for index in range(manifest["rounds"]) for name in variants}
    seen_runs = set()
    observations = []
    for run in manifest["runs"]:
        key = (run["variant"], run["sample"])
        if key not in expected_runs or key in seen_runs:
            raise ValueError("Unexpected or duplicate measurement run")
        seen_runs.add(key)
        raw = output / run["raw"]
        if digest(raw) != run["raw_sha256"]:
            raise ValueError("Raw benchmark output changed")
        rows = parse_benchmarks(raw.read_text())
        if sorted(row["case"] for row in rows) != variants[run["variant"]]["cases"]:
            raise ValueError("Missing or unexpected benchmark cases")
        observations.extend({**row, "variant": run["variant"], "sample": run["sample"]}
                            for row in rows)
    if seen_runs != expected_runs:
        raise ValueError("Missing measurement runs")
    if json.loads((output / "observations.json").read_text()) != observations:
        raise ValueError("Derived observations do not match raw benchmark output")
    groups = {}
    for row in observations:
        groups.setdefault((row["variant"], row["case"]), []).append(row)
    summary = []
    for (variant, case), rows in sorted(groups.items()):
        values = [r["ns_per_op"] for r in rows]
        median = statistics.median(values)
        mad = statistics.median(abs(v - median) for v in values)
        summary.append({
            "variant": variant, "case": case, "n": len(rows), "median_ns": median,
            "min_ns": min(values), "max_ns": max(values), "mad_pct": 100*mad/median,
            "range_pct": 100*(max(values)-min(values))/median,
            "median_bytes": statistics.median(r["bytes_per_op"] for r in rows),
            "median_allocs": statistics.median(r["allocs_per_op"] for r in rows),
            "matched_output_group": case not in manifest["config"]["separate_cases"],
        })
    atomic_json(output / "summary.json", summary)
    for group in ("matched", "controls"):
        folder = output / "benchstat" / group
        folder.mkdir(parents=True, exist_ok=True)
        for variant in manifest["variants"]:
            chunks = []
            for run in manifest["runs"]:
                if run["variant"] != variant["name"]:
                    continue
                raw = output / run["raw"]
                if digest(raw) != run["raw_sha256"]:
                    raise ValueError("Raw benchmark output changed")
                for line in raw.read_text().splitlines():
                    if line.startswith(("goos:", "goarch:", "pkg:", "cpu:")):
                        chunks.append(line)
                    elif line.startswith("BenchmarkRequest/"):
                        row = parse_benchmarks(line)[0]
                        is_control = row["case"] in manifest["config"]["separate_cases"]
                        if (group == "controls") == is_control:
                            chunks.append(line)
            (folder / f"{variant['name']}.txt").write_text("\n".join(chunks) + "\n")
    print(f"Validated {len(observations)} observations; summaries and separated benchstat inputs written")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    prep = sub.add_parser("prepare")
    prep.add_argument("output", type=Path)
    run = sub.add_parser("sample")
    run.add_argument("output", type=Path)
    run.add_argument("--rounds", type=int, default=12)
    run.add_argument("--benchtime", default="500ms")
    run.add_argument("--cpu", type=int)
    run.add_argument("--seed", type=int, default=2714)
    report = sub.add_parser("summarize")
    report.add_argument("output", type=Path)
    args = parser.parse_args()
    output = args.output.expanduser().resolve()
    if args.action == "prepare":
        prepare(output)
    elif args.action == "sample":
        sample(output, args.rounds, args.benchtime, args.cpu, args.seed)
    else:
        summarize(output)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        raise SystemExit(1)
