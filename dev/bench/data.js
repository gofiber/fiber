window.BENCHMARK_DATA = {
  "lastUpdate": 1589324744281,
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
      },
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
          "id": "a08dbd5c5bb1e621deb8e7deb621d846f3473913",
          "message": "Update ctx_bench_test.go",
          "timestamp": "2020-05-13T00:49:16+02:00",
          "tree_id": "edcf1c62ed4ab79555a6816266166b74fe3efd05",
          "url": "https://github.com/Fenny/fiber/commit/a08dbd5c5bb1e621deb8e7deb621d846f3473913"
        },
        "date": 1589323846280,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 292,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3946101 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 199,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6153844 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 251,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4570456 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 284,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4320252 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15050121 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50611675 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "58254507 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 75.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16142785 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 170,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7027626 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 627,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1867317 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7934844 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 3331,
            "unit": "ns/op\t    4560 B/op\t      13 allocs/op",
            "extra": "356917 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 522,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2217397 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 692,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1756056 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 211,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5770669 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 118,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10201286 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "64924254 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.43,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "140645640 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38012468 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 67.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17606474 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20647111 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14319768 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 280,
            "unit": "ns/op\t     148 B/op\t       3 allocs/op",
            "extra": "4211251 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11480682 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4695,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "251214 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1219828,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "964 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5354,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "218956 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1961,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "614220 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5407,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "227842 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5064,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "244016 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 204,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5666437 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 66.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18062726 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.01,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "397818808 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35128322 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.92,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "405816981 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 47.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26080159 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 156,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7708993 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 49.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25107871 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 123,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9901179 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.398,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      },
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
          "id": "d729bb8cd5bb999aba1168a2003b742dbb3fed7d",
          "message": "Merge remote-tracking branch 'upstream/master'",
          "timestamp": "2020-05-13T01:00:31+02:00",
          "tree_id": "92960f695caa215d3d4e1d49831c8f74c50ccef4",
          "url": "https://github.com/Fenny/fiber/commit/d729bb8cd5bb999aba1168a2003b742dbb3fed7d"
        },
        "date": 1589324515430,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 294,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3934768 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 193,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6235572 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 248,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5044651 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 270,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4526960 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 78.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15688042 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49804064 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65739685 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 70.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17365068 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 155,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7575547 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 634,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1897298 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7550622 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 3402,
            "unit": "ns/op\t    4560 B/op\t      13 allocs/op",
            "extra": "356181 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 549,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2230045 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 699,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1716514 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 209,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5773508 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11585065 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 19.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65395432 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "137965012 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "33471674 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20070476 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 49.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24031042 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13639932 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 329,
            "unit": "ns/op\t     138 B/op\t       3 allocs/op",
            "extra": "3746227 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11401116 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 6333,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "175070 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1785581,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "656 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 7379,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "156816 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2453,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "447757 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 7308,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "162411 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 6335,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "184497 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 202,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5803266 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 56.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21158419 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.07,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "372215372 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "33495092 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "379370745 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27564562 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 132,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8904354 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 41.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28810590 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 91,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13243081 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      },
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
          "id": "5c448a5597e7c2ea45fb7f79a82df0a718db100d",
          "message": "Remove benchmark data",
          "timestamp": "2020-05-13T01:04:06+02:00",
          "tree_id": "a5e4c3a169986ededbb4b032e4d12633693a89a1",
          "url": "https://github.com/Fenny/fiber/commit/5c448a5597e7c2ea45fb7f79a82df0a718db100d"
        },
        "date": 1589324742888,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 292,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4014512 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 201,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5989942 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 247,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4792009 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 276,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3985563 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 82.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14930935 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "47652547 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 21.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57651598 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 74.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16172712 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 172,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7069797 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 641,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1897509 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7711971 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 3369,
            "unit": "ns/op\t    4560 B/op\t      13 allocs/op",
            "extra": "337618 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 531,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2244242 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 692,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1723137 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 224,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5051066 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 119,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10264890 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "66212598 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "142640925 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37457877 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 67.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17843104 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 59.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21070644 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14424123 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 283,
            "unit": "ns/op\t     148 B/op\t       3 allocs/op",
            "extra": "4220209 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11493696 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4745,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "257158 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1231948,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "978 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5530,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220243 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2062,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "592287 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5455,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "222031 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5008,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "244111 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 202,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5728316 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 67.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18235342 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.04,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "395278048 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35251311 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.94,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "412623562 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 47.4,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25787606 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 153,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7765927 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 52.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22961242 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11796699 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      }
    ]
  }
}