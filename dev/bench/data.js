window.BENCHMARK_DATA = {
  "lastUpdate": 1589323612676,
  "repoUrl": "https://github.com/Fenny/fiber",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "25108519+Fenny@users.noreply.github.com",
            "name": "Fenny",
            "username": "Fenny"
          },
          "committer": {
            "email": "25108519+Fenny@users.noreply.github.com",
            "name": "Fenny",
            "username": "Fenny"
          },
          "distinct": true,
          "id": "991344f0329336ee320fb17c7b8f9ee2af9eaaae",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T00:45:16+02:00",
          "tree_id": "cae0b8a6b1c2ce5074ce27e24d7df6d18c6fa935",
          "url": "https://github.com/Fenny/fiber/commit/991344f0329336ee320fb17c7b8f9ee2af9eaaae"
        },
        "date": 1589323611591,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 276,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4061066 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 185,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6635655 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 235,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4797228 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 252,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "5031640 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 76.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14410456 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50307316 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 17.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65889913 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 69.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18557587 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 148,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8062204 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 583,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2054630 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 137,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8225664 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 3434,
            "unit": "ns/op\t    4560 B/op\t      13 allocs/op",
            "extra": "372649 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 510,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2259499 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 659,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1753622 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 184,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6530884 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 97.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13715415 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 700,
            "unit": "ns/op\t      58 B/op\t       6 allocs/op",
            "extra": "1740868 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 41.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "28171048 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.34,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "152408205 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 30.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41001295 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 53.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21760610 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 41.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27005001 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 73.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15418033 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 319,
            "unit": "ns/op\t     153 B/op\t       3 allocs/op",
            "extra": "4020853 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 94.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12703977 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5586,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "204673 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1654754,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "706 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 6217,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "190170 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2036,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "521929 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 6248,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "191542 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5421,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "225084 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 171,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7029970 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 48.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22590822 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.59,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "471936325 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 28.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "38867258 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "442337551 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 37.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "30995876 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 113,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10083229 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 38.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "30921890 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 81.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13710102 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.374,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      }
    ]
  }
}