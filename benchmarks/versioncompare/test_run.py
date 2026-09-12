import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("versioncompare", Path(__file__).with_name("run.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class RunnerTests(unittest.TestCase):
    def test_json_stream(self):
        self.assertEqual(runner.json_stream('{"a":1}\n{"b":2}\n'), [{"a": 1}, {"b": 2}])

    def test_balanced_orders(self):
        orders = runner.balanced_orders(["v2", "v3", "snapshot"], 12, 2714)
        self.assertEqual(orders, runner.balanced_orders(["v2", "v3", "snapshot"], 12, 2714))
        for position in range(3):
            self.assertEqual({name: sum(order[position] == name for order in orders)
                              for name in orders[0]}, {"v2": 4, "v3": 4, "snapshot": 4})

    def test_unbalanced_round_count_rejected(self):
        with self.assertRaises(ValueError):
            runner.balanced_orders(["v2", "v3", "snapshot"], 5, 2714)

    def test_parse_benchmark(self):
        rows = runner.parse_benchmarks("BenchmarkRequest/static-8 1000 120.5 ns/op 16 B/op 1 allocs/op\n")
        self.assertEqual(rows, [{"case": "static", "iterations": 1000, "ns_per_op": 120.5,
                                 "bytes_per_op": 16.0, "allocs_per_op": 1.0}])

    def test_duplicate_case_rejected(self):
        line = "BenchmarkRequest/static 100 100 ns/op 0 B/op 0 allocs/op\n"
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            runner.parse_benchmarks(line + line)

    def test_invalid_units_rejected(self):
        with self.assertRaises(ValueError):
            runner.parse_benchmarks("BenchmarkRequest/static 100 100 us/op 0 B/op 0 allocs/op")

    def test_invalid_numbers_rejected(self):
        for metric in range(3):
            for bad in ("nan", "inf", "-inf", "-1"):
                with self.subTest(metric=metric, value=bad):
                    values = ["100", "16", "1"]
                    values[metric] = bad
                    line = (f"BenchmarkRequest/static 100 {values[0]} ns/op "
                            f"{values[1]} B/op {values[2]} allocs/op")
                    with self.assertRaises(ValueError):
                        runner.parse_benchmarks(line)

    def test_environment_ignores_persistent_go_settings(self):
        env = runner.environment("go1.27.0")
        self.assertEqual(env["GOENV"], "off")
        self.assertEqual(env["GOWORK"], "off")
        self.assertNotIn("GOFLAGS", env)

    def test_template_adapts_native_apis(self):
        template = "__FIBER__ __CTX__ __BODY_BIND__ __QUERY_BIND__ __ORIGINS__ __METHODS__ __METHODS_HEADER__ __NOT_FOUND__"
        v2 = runner.render(template, {"major": 2, "module": "github.com/gofiber/fiber/v2"})
        v3 = runner.render(template, {"major": 3, "module": "github.com/gofiber/fiber/v3"})
        self.assertIn("*fiber.Ctx", v2)
        self.assertIn("c.BodyParser(&p)", v2)
        self.assertIn("c.Bind().Body(&p)", v3)
        self.assertNotIn("__", v2 + v3)

    def test_unknown_template_token_rejected(self):
        with self.assertRaises(ValueError):
            runner.render("__UNEXPECTED__", {"major": 3, "module": "example"})

    def test_incomplete_measurement_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            runner.atomic_json(root / "manifest.json", {"phase": "sample_failed"})
            with self.assertRaisesRegex(ValueError, "Incomplete"):
                runner.summarize(root)

    def measurement(self, root):
        text = "BenchmarkRequest/static 100 100 ns/op 0 B/op 0 allocs/op\n"
        raw = root / "raw.txt"
        raw.write_text(text)
        manifest = {
            "phase": "complete", "rounds": 1, "config": {"separate_cases": []},
            "variants": [{"name": "v2", "cases": ["static"]}],
            "runs": [{"variant": "v2", "sample": 0, "raw": "raw.txt",
                      "raw_sha256": runner.digest(raw)}],
        }
        observations = [{**runner.parse_benchmarks(text)[0], "variant": "v2", "sample": 0}]
        runner.atomic_json(root / "manifest.json", manifest)
        runner.atomic_json(root / "observations.json", observations)
        return manifest, observations

    def test_complete_measurement_summarized(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.measurement(root)
            runner.summarize(root)
            summary = json.loads((root / "summary.json").read_text())
            self.assertEqual(summary[0]["median_ns"], 100)
            self.assertIn("BenchmarkRequest/static", (root / "benchstat/matched/v2.txt").read_text())
            self.assertNotIn("BenchmarkRequest/static", (root / "benchstat/controls/v2.txt").read_text())

    def test_changed_raw_measurement_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.measurement(root)
            (root / "raw.txt").write_text("altered")
            with self.assertRaisesRegex(ValueError, "Raw benchmark output changed"):
                runner.summarize(root)
            self.assertFalse((root / "summary.json").exists())

    def test_changed_derived_measurement_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, observations = self.measurement(root)
            observations[0]["ns_per_op"] = 1
            runner.atomic_json(root / "observations.json", observations)
            with self.assertRaisesRegex(ValueError, "Derived observations"):
                runner.summarize(root)
            self.assertFalse((root / "summary.json").exists())

    def test_missing_and_duplicate_runs_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, observations = self.measurement(root)
            for runs in ([], manifest["runs"] * 2):
                with self.subTest(runs=runs):
                    runner.atomic_json(root / "manifest.json", {**manifest, "runs": runs})
                    with self.assertRaises(ValueError):
                        runner.summarize(root)

    def test_missing_case_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, observations = self.measurement(root)
            manifest["variants"][0]["cases"].append("another")
            runner.atomic_json(root / "manifest.json", manifest)
            with self.assertRaisesRegex(ValueError, "Missing or unexpected"):
                runner.summarize(root)

    def test_prepare_preserves_existing_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            marker = root / "keep"
            marker.write_text("existing")
            with self.assertRaises(FileExistsError):
                runner.prepare(root)
            self.assertEqual(marker.read_text(), "existing")


if __name__ == "__main__":
    unittest.main()
