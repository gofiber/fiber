window.BENCHMARK_DATA = {
  "lastUpdate": 1590221215647,
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
          "id": "a327b3f3568baba916dcbe0e465046ab950b4648",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:03:54+02:00",
          "tree_id": "864faa5b4e1c60eee7a232d6a41dff035d05abef",
          "url": "https://github.com/Fenny/fiber/commit/a327b3f3568baba916dcbe0e465046ab950b4648"
        },
        "date": 1590221127681,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 5908,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "201554 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 148,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8046382 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 66.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17356180 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 87.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13189620 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 70.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18025490 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 308,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3887755 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 93.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13916310 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 138,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7894350 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 145,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8788046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 142,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8178201 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 492,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2451261 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1832,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "726921 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 191,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6408763 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 122,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9348828 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 51.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21001200 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 26.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "45884270 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 136,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8740178 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 460,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2529094 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 606,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2011128 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 231,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5120035 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 239,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4547926 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27742423 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 125,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9352356 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 290,
            "unit": "ns/op\t     268 B/op\t       4 allocs/op",
            "extra": "5058030 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1219,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "834403 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1183,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 256,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "4781695 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1070,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 60.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17040109 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "900676 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1164,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "971652 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 967815,
            "unit": "ns/op\t     157 B/op\t       2 allocs/op",
            "extra": "1279 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 5662,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "197924 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 5825,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "208052 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 161,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "7448835 times\n2 procs"
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
          "id": "aec1bfccf0feb8b46a32fd5346ae03718557cea2",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:19+02:00",
          "tree_id": "104f8921842b61b37ca5c3ff53d9d9e2a289d1fe",
          "url": "https://github.com/Fenny/fiber/commit/aec1bfccf0feb8b46a32fd5346ae03718557cea2"
        },
        "date": 1590221158280,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 6882,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "174276 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 173,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6944904 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 78,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15359299 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11851038 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 81.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14980516 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 346,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3476216 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 104,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "11686802 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 159,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7630262 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7259845 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 173,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7401229 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 556,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2164884 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2086,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "501114 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 213,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5705665 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 145,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8344276 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 62.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18665475 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 30.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38710495 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 150,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8039445 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 510,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2316861 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 689,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1759981 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 262,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4757036 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 277,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4455441 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 47.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24284116 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 146,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8240382 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 312,
            "unit": "ns/op\t     248 B/op\t       4 allocs/op",
            "extra": "4479597 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1390,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "743244 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1331,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "904795 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 295,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "4058607 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1227,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "904572 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 70.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16416525 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1309,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "908965 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1318,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "864550 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1098255,
            "unit": "ns/op\t     182 B/op\t       2 allocs/op",
            "extra": "1100 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 6648,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "173996 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 6771,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "179304 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 187,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6787734 times\n2 procs"
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
          "id": "c756ad1a683e9f2efcbb3fddfb8f9b906b695c8d",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:25+02:00",
          "tree_id": "864faa5b4e1c60eee7a232d6a41dff035d05abef",
          "url": "https://github.com/Fenny/fiber/commit/c756ad1a683e9f2efcbb3fddfb8f9b906b695c8d"
        },
        "date": 1590221166147,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 6594,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "174668 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 192,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6116889 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 83.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13760398 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 113,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10824260 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 87.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13610191 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 383,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3192066 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 114,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "11061216 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 169,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7133569 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 178,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6735625 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 176,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6590764 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 608,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2003412 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2402,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "548586 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 239,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4970832 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 169,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7370757 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16733985 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 31.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38619174 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 173,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7006621 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 533,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2213222 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 686,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1752673 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 283,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4395303 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 286,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4125919 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 57.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20597263 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7456862 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 306,
            "unit": "ns/op\t     242 B/op\t       4 allocs/op",
            "extra": "4646276 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1394,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "763916 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1345,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "873414 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 324,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "3637936 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1271,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "972121 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16604815 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1347,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "861614 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1325,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "836100 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1103445,
            "unit": "ns/op\t     196 B/op\t       2 allocs/op",
            "extra": "1020 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 6540,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "182323 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 6805,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "170246 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 202,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6304389 times\n2 procs"
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
          "id": "dca25b9acb7f441c2ef776b3efcd503f9bfe4fa6",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:34+02:00",
          "tree_id": "104f8921842b61b37ca5c3ff53d9d9e2a289d1fe",
          "url": "https://github.com/Fenny/fiber/commit/dca25b9acb7f441c2ef776b3efcd503f9bfe4fa6"
        },
        "date": 1590221168681,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 5466,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "200353 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 155,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7582100 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 67.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15226286 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 96.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11941981 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 71.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15881466 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 296,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "4140196 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 101,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13982180 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 135,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9005012 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 147,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8170423 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 139,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8270665 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 504,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2461436 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2003,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "702424 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 198,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6058598 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 138,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8076579 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 56.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20134260 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 26,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "44529176 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 138,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "9143799 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 443,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2591816 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 568,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2136828 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 255,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5159176 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 225,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4745187 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 49.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24922525 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 127,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9272284 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 258,
            "unit": "ns/op\t     242 B/op\t       4 allocs/op",
            "extra": "5805948 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1163,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000431 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1133,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 268,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "4430234 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1027,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 53.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21126986 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1136,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1117782 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 907109,
            "unit": "ns/op\t     150 B/op\t       2 allocs/op",
            "extra": "1334 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 5467,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "209133 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 5324,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "217870 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 169,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6957451 times\n2 procs"
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
          "id": "7ae76de074d8616369bf390ca63a8aa4d4aaa002",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:39+02:00",
          "tree_id": "864faa5b4e1c60eee7a232d6a41dff035d05abef",
          "url": "https://github.com/Fenny/fiber/commit/7ae76de074d8616369bf390ca63a8aa4d4aaa002"
        },
        "date": 1590221184802,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 6825,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "170893 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 176,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6817038 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15494589 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11824790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 80.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15046272 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 344,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3415024 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 110,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "11769265 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 159,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7522387 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7497993 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7564983 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 565,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2181003 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2087,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "586856 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 213,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5501140 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 145,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8408938 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 62.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19372512 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 30.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39796776 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 153,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7701271 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 507,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2354757 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 690,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1749742 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 262,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4547433 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 268,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4452776 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 48.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25071849 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 146,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8092066 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 309,
            "unit": "ns/op\t     242 B/op\t       4 allocs/op",
            "extra": "4625116 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1348,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "873399 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1294,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "874696 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 294,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "4151847 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1211,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "974406 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 69.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16901713 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1285,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "908643 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1285,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "900694 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1101918,
            "unit": "ns/op\t     190 B/op\t       2 allocs/op",
            "extra": "1056 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 6588,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "179948 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 6659,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "177840 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 185,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6558902 times\n2 procs"
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
          "id": "5ebb13d98c5485c0bdc9fc70b490dbeb09f93257",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:45+02:00",
          "tree_id": "104f8921842b61b37ca5c3ff53d9d9e2a289d1fe",
          "url": "https://github.com/Fenny/fiber/commit/5ebb13d98c5485c0bdc9fc70b490dbeb09f93257"
        },
        "date": 1590221186635,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 6599,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "185773 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 190,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6300045 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 85.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14124558 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 111,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10695285 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 83.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14471541 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 370,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3100953 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 113,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "10894684 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7355325 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 182,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6710812 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 167,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7073095 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 566,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2106282 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2170,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "639321 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 226,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5172242 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7490973 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 69.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15214497 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 31.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38092396 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 171,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7232805 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 530,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2285128 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 681,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1727148 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 270,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4551552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 288,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4145827 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 60.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19911072 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 156,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7640660 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 289,
            "unit": "ns/op\t     239 B/op\t       4 allocs/op",
            "extra": "4703804 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1359,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "795535 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1328,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "889792 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 331,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "3685837 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1227,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "920563 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 64.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17452468 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1273,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "919010 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1271,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "891787 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1092261,
            "unit": "ns/op\t     184 B/op\t       2 allocs/op",
            "extra": "1090 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 6372,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "185662 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 6302,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "173372 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 197,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6122710 times\n2 procs"
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
          "id": "7b5180e8b2557ae3e88e853abea22a5633bd4bb6",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:04:54+02:00",
          "tree_id": "864faa5b4e1c60eee7a232d6a41dff035d05abef",
          "url": "https://github.com/Fenny/fiber/commit/7b5180e8b2557ae3e88e853abea22a5633bd4bb6"
        },
        "date": 1590221194710,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 5980,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "181789 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 180,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6355780 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 78.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14838772 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11533654 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 77.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14738552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 340,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3573669 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 106,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "11653846 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7388038 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 165,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7085872 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7610882 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 555,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2153790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2010,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "672686 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 213,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5494740 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 156,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7153735 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 65.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17829296 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 30.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39065472 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 158,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7494667 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 500,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2422608 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 644,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1824772 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 252,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4633720 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 264,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4425530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 56.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21305120 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 147,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7797597 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 287,
            "unit": "ns/op\t     244 B/op\t       4 allocs/op",
            "extra": "4595220 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1289,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "841302 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1221,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "919922 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 320,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "3895849 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 62,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18851499 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1198,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "902569 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1222,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "922978 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1025635,
            "unit": "ns/op\t     170 B/op\t       2 allocs/op",
            "extra": "1176 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 5741,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "193251 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 5929,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "201074 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 188,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6631088 times\n2 procs"
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
          "id": "3f316b21e53b92d72d5eb32a3f977754ba146157",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:05:04+02:00",
          "tree_id": "104f8921842b61b37ca5c3ff53d9d9e2a289d1fe",
          "url": "https://github.com/Fenny/fiber/commit/3f316b21e53b92d72d5eb32a3f977754ba146157"
        },
        "date": 1590221198644,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 5814,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "197121 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 147,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8107976 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 66.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18491883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 88.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14052360 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 68.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18192164 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 303,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3876772 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 89,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13098943 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 135,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8793972 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 138,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8660469 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 139,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9313813 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 480,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2550097 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1881,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "804637 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 187,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6356600 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 125,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9394983 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 53.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23393176 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 25.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42451184 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 131,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "9206106 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 447,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2653251 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 583,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1838612 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 220,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5191544 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 233,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4881944 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29558216 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 124,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10260262 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 271,
            "unit": "ns/op\t     257 B/op\t       4 allocs/op",
            "extra": "5346679 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1215,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "884662 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1138,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 245,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "5050186 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1035,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 59.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18937914 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1182,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "988498 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1142,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "988124 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 968462,
            "unit": "ns/op\t     155 B/op\t       2 allocs/op",
            "extra": "1293 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 5805,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "187167 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 5753,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "201064 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 164,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "7750779 times\n2 procs"
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
          "id": "0bc84eb91cd8d1dca85f2b2a0403f4686a1c77a4",
          "message": "Trigger benchmarks",
          "timestamp": "2020-05-23T10:05:16+02:00",
          "tree_id": "864faa5b4e1c60eee7a232d6a41dff035d05abef",
          "url": "https://github.com/Fenny/fiber/commit/0bc84eb91cd8d1dca85f2b2a0403f4686a1c77a4"
        },
        "date": 1590221214701,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_App_ETag",
            "value": 6588,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "187134 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 150,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7542622 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 67.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14959532 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 92.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13859097 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 75.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15973107 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 311,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "3658844 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 96.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "12668262 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7943338 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 155,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8131251 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7688781 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 522,
            "unit": "ns/op\t      32 B/op\t       2 allocs/op",
            "extra": "2200461 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1978,
            "unit": "ns/op\t    4464 B/op\t       7 allocs/op",
            "extra": "694582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_IPs",
            "value": 200,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6027873 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Is",
            "value": 141,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8887676 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 60.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19841587 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Protocol",
            "value": 28.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40547031 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 143,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8411968 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 448,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2456678 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 594,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2006151 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 244,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5076552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 258,
            "unit": "ns/op\t      64 B/op\t       4 allocs/op",
            "extra": "4416183 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 45.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27050380 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 133,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8866191 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 299,
            "unit": "ns/op\t     241 B/op\t       4 allocs/op",
            "extra": "4656859 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1355,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "768421 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_Strict_Case",
            "value": 1233,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "970714 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Chain",
            "value": 276,
            "unit": "ns/op\t       1 B/op\t       1 allocs/op",
            "extra": "4270426 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next",
            "value": 1207,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "965870 times\n2 procs"
          },
          {
            "name": "Benchmark_Route_Match",
            "value": 68.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16939849 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_CaseSensitive",
            "value": 1187,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "910959 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler_StrictRouting",
            "value": 1248,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "928224 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1083993,
            "unit": "ns/op\t     186 B/op\t       2 allocs/op",
            "extra": "1077 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag",
            "value": 6600,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "194910 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_ETag_Weak",
            "value": 7739,
            "unit": "ns/op\t    1056 B/op\t       4 allocs/op",
            "extra": "174814 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 180,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "6260184 times\n2 procs"
          }
        ]
      }
    ]
  }
}