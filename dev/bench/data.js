window.BENCHMARK_DATA = {
  "lastUpdate": 1589581551378,
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
          "id": "d77342001ad46353854997d9e720a6b65c2840f4",
          "message": "Update linter.yml",
          "timestamp": "2020-05-15T23:56:31+02:00",
          "tree_id": "9f11f5ebbb9a286cb30679d1aacf5fe2aeadf62d",
          "url": "https://github.com/Fenny/fiber/commit/d77342001ad46353854997d9e720a6b65c2840f4"
        },
        "date": 1589579878618,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 297,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4033539 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 211,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5825240 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 261,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3917994 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 288,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3940046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 668,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1764046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 90.5,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14263069 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 178,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6700192 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 308,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3819723 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 262,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4572308 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 690,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1696147 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2422,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "421971 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 740,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1661787 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 221,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5223936 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 76.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15360338 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1098,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 174,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6926432 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 54.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22794453 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 85.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13592346 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 394,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "3046566 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 109,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "10799186 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1156,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41936203 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5248,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "226926 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1306647,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "926 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5391,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "190813 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2054,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "568050 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5471,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "219240 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5199,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "234154 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 256,
            "unit": "ns/op\t      96 B/op\t       2 allocs/op",
            "extra": "4807244 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 67.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17470353 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 53.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22390694 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10896699 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.637,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 107,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11206114 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 106,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "10838347 times\n2 procs"
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
          "id": "3884148d23f2ce65beb4fc1f8b9aff5c6d6134a9",
          "message": "Update linter.yml",
          "timestamp": "2020-05-16T00:03:23+02:00",
          "tree_id": "aa512c44e0dcb5f82add59559f305eed9efae2de",
          "url": "https://github.com/Fenny/fiber/commit/3884148d23f2ce65beb4fc1f8b9aff5c6d6134a9"
        },
        "date": 1589580296121,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 294,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4110741 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 192,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5988758 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 250,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4755752 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 276,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3917743 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 622,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1868428 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16158982 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7858628 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 303,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3926205 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 274,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4681764 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 670,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1807704 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2337,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "491397 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 679,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1687378 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 221,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5784883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 72.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15927199 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1007,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 172,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6497406 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 58.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20237655 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 88.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13294426 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 366,
            "unit": "ns/op\t     222 B/op\t       4 allocs/op",
            "extra": "3341268 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 98.7,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11591956 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1076,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "45146617 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4965,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "238123 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1241214,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "962 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5286,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "242827 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1931,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "602318 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5035,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "226632 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4923,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "245798 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 231,
            "unit": "ns/op\t      96 B/op\t       2 allocs/op",
            "extra": "5018714 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 67.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17555524 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 51.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22885776 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 109,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11110635 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.557,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 98.5,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12446061 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 98.8,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12003196 times\n2 procs"
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
          "id": "6a734fb46f6ef9e9bd982a45273f735ac4df3d12",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:04:02+02:00",
          "tree_id": "07df9592c7ac2db9c2a292417354b772c0c780df",
          "url": "https://github.com/Fenny/fiber/commit/6a734fb46f6ef9e9bd982a45273f735ac4df3d12"
        },
        "date": 1589580337665,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 306,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3828070 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 212,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5563708 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 261,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4379835 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 287,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4048075 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 669,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1777137 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 89,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13773270 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 170,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7112497 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 308,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3990076 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 270,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4451222 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 687,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1786599 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2664,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "473778 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 735,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1601689 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 230,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4800127 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 77.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15373412 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1035,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 186,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6414136 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 56.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21067467 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 87.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13452445 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 378,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "3046780 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 112,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11269464 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1165,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38761515 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5156,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220014 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1335199,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "943 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5555,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "210057 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2096,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "583280 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5734,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "187106 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5201,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "216362 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 300,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4159182 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 69.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16961480 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 52.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22635217 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 112,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10815858 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.624,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 112,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "10882557 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 112,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11085093 times\n2 procs"
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
          "id": "5b725458e571506dc4979914e61ee6cedad6ae97",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:23:44+02:00",
          "tree_id": "649686f09bf0f4e497934f3c1170abba0e39716b",
          "url": "https://github.com/Fenny/fiber/commit/5b725458e571506dc4979914e61ee6cedad6ae97"
        },
        "date": 1589581514497,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 293,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4051724 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 193,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6311071 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 242,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4770544 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 260,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4645442 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 635,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1887546 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 78.1,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15450772 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 160,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7355445 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 276,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4349678 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 228,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5278503 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 642,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1787004 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2201,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "542883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 706,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1661946 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 202,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5688408 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17661334 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 969,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1219785 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 154,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7110606 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 44.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27048182 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 81.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14677384 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 358,
            "unit": "ns/op\t     212 B/op\t       4 allocs/op",
            "extra": "2854152 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 89.5,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13725448 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1068,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "46694905 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5142,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "217002 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1586073,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "766 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5808,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "206178 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "606163 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5684,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "213818 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4969,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "241536 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 264,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4507246 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 58.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20799512 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 44.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27437826 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 87.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13678411 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.611,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 88.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13476622 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 90.1,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13458741 times\n2 procs"
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
          "id": "e58ca1091364ee2833d680c6665bb1eb45212406",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:23:50+02:00",
          "tree_id": "07df9592c7ac2db9c2a292417354b772c0c780df",
          "url": "https://github.com/Fenny/fiber/commit/e58ca1091364ee2833d680c6665bb1eb45212406"
        },
        "date": 1589581523356,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 291,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4058266 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 182,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6344232 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4820071 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 257,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4666848 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 618,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1961269 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 77.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16382338 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7237365 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 273,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4130434 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 223,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5368532 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 622,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1938156 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2146,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "566736 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 720,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1683290 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 216,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5948392 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 69.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16823378 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 988,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1230698 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 153,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6637495 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 41.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28568430 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 76.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15380002 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 386,
            "unit": "ns/op\t     233 B/op\t       4 allocs/op",
            "extra": "3118902 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 87.9,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13823516 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1000,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1207293 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 21.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57536457 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4805,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "259323 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1463597,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "795 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5296,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "217434 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1854,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "619202 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5450,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "226935 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4813,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "240193 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 258,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4600681 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21969458 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 42.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28319814 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 84.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14486872 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.607,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 86.6,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14148506 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 87.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14370620 times\n2 procs"
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
          "id": "873f1eea637e1c47a8cbef46cda77b2071e39556",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:24:04+02:00",
          "tree_id": "2ec2ed0ba570e2e416e337d02c0d7e91f30363d3",
          "url": "https://github.com/Fenny/fiber/commit/873f1eea637e1c47a8cbef46cda77b2071e39556"
        },
        "date": 1589581534482,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 273,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4324969 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 185,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6601675 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4805749 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 252,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4820428 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 611,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1948658 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 75.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14505334 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 157,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7367882 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 271,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4433017 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 222,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5417025 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 625,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1930824 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2088,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "563089 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 674,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1767624 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 201,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5887960 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17763849 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 985,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1210353 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 151,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8047748 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 44.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25454104 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 79.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14992327 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 376,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "3054740 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 84.8,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14273684 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 974,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1252093 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54188721 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4957,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "238050 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1558098,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "770 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5502,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "212043 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1922,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "620353 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5646,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "218325 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4667,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "242325 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 253,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4700964 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21345777 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 40.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29958240 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 83.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14051510 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.587,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 83.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12934095 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 85,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14682040 times\n2 procs"
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
          "id": "ac910c31143eb650f043ea56526b239d64f171b0",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:24:14+02:00",
          "tree_id": "07df9592c7ac2db9c2a292417354b772c0c780df",
          "url": "https://github.com/Fenny/fiber/commit/ac910c31143eb650f043ea56526b239d64f171b0"
        },
        "date": 1589581550427,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 287,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3736300 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 194,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6079779 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 249,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4886685 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 270,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4514542 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 631,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1895811 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 83,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15014134 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 173,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7238613 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 292,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4163802 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 256,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4581534 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 634,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1850076 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2431,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "483399 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 686,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1686259 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 221,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5527484 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 74.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15913986 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1037,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 168,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7287499 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 53.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22152486 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 86.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13141329 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 351,
            "unit": "ns/op\t     221 B/op\t       4 allocs/op",
            "extra": "3363633 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 104,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12609654 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1034,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 26.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "43248363 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4845,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "246550 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1188705,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1024 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5186,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "237178 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2029,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "588464 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5265,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "226872 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4985,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "251150 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 271,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4162584 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 65.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18700456 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 56.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21206899 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 98.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12058444 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.586,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 101,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11895090 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 97,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12751645 times\n2 procs"
          }
        ]
      }
    ]
  }
}