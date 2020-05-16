window.BENCHMARK_DATA = {
  "lastUpdate": 1589597201571,
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
          "id": "bc9ee7eeccbd8b888d0876443c46809d75fa24b8",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:26:30+02:00",
          "tree_id": "2ec2ed0ba570e2e416e337d02c0d7e91f30363d3",
          "url": "https://github.com/Fenny/fiber/commit/bc9ee7eeccbd8b888d0876443c46809d75fa24b8"
        },
        "date": 1589581679755,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 285,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4314529 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 191,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5974856 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 242,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5034746 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 253,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4842394 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 630,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1919319 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.1,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14955642 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7687608 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 274,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4532668 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 228,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5329825 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 645,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1828039 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2459,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "516363 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 696,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1692110 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 206,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5875582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16767906 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1005,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 155,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7535288 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27266217 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 82.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12953457 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 369,
            "unit": "ns/op\t     213 B/op\t       4 allocs/op",
            "extra": "2835510 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 90.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12255780 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1067,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50970594 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5121,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "218924 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1582493,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "676 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5695,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "216585 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1983,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "603656 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5800,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220152 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4997,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "242956 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 272,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4195970 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21112788 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 44.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26783307 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 89.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13640023 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.617,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 91.4,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13149213 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 89.6,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13624006 times\n2 procs"
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
          "id": "ae626745409e8fc03be0b7d63dfa82cea54af1d8",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:26:35+02:00",
          "tree_id": "07df9592c7ac2db9c2a292417354b772c0c780df",
          "url": "https://github.com/Fenny/fiber/commit/ae626745409e8fc03be0b7d63dfa82cea54af1d8"
        },
        "date": 1589581685502,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 275,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4356315 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 176,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6925525 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 228,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5176700 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 237,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4863828 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 578,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2003984 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 73.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "17185484 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 148,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7834483 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 257,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4525551 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 205,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5642562 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 569,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2028964 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2120,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "566125 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 662,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1802414 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 186,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6305727 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 61.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19953644 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 906,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1358098 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 149,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7626058 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 39.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31173400 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 73.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15952191 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 382,
            "unit": "ns/op\t     229 B/op\t       4 allocs/op",
            "extra": "3204552 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 85.8,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14108827 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1043,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54390112 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4926,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "237727 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1521791,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "783 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5533,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "209793 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1792,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "613785 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5457,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "233809 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4791,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "233162 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 259,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4643036 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 52.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23005448 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 39.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "30288619 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 82.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14805640 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.572,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 83.8,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "15131157 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 86.2,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13052806 times\n2 procs"
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
          "id": "c89ea5b26d769ee1ff3dbe3406fcbe5f36d0d462",
          "message": "Update utils_bench_test.go",
          "timestamp": "2020-05-16T00:26:45+02:00",
          "tree_id": "2ec2ed0ba570e2e416e337d02c0d7e91f30363d3",
          "url": "https://github.com/Fenny/fiber/commit/c89ea5b26d769ee1ff3dbe3406fcbe5f36d0d462"
        },
        "date": 1589581720027,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 297,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3827583 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 202,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5876121 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 248,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4539769 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 270,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4331607 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 642,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1899873 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 85.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14613568 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 171,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7172155 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 301,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3944608 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 256,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4579258 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 663,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1804797 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2374,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "488984 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 719,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1635030 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 221,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5594136 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 70.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16942476 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1043,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 171,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6855441 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 52.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22171186 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14494941 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 368,
            "unit": "ns/op\t     230 B/op\t       4 allocs/op",
            "extra": "3177160 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 103,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11149123 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1127,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "43262990 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5065,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "231478 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1273864,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "964 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5558,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "216402 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2079,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "574027 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5381,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "218972 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5057,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "246427 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 276,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4321962 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 65.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18032378 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 50,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23907760 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 111,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10334250 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.626,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 102,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11921025 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 100,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11398191 times\n2 procs"
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
          "id": "4a1507e9a4468dd2d74c78344660368a668bdd8b",
          "message": "Fix tests",
          "timestamp": "2020-05-16T00:45:47+02:00",
          "tree_id": "789500da3ea67bd9f84255480ad750517f02d485",
          "url": "https://github.com/Fenny/fiber/commit/4a1507e9a4468dd2d74c78344660368a668bdd8b"
        },
        "date": 1589582837016,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 311,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3720523 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 208,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5752227 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 264,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4604492 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 284,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4170339 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 659,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1842750 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 84.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14167976 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 164,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7506714 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 311,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3897892 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 265,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4497903 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 666,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1792489 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2413,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "462327 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 746,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1649629 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 229,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5194345 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 76.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15837051 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1018,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 172,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7185680 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 58.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19059121 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 86.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13287901 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 392,
            "unit": "ns/op\t     243 B/op\t       4 allocs/op",
            "extra": "2951727 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 66.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17737321 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 990,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1207136 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41911862 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4843,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "249552 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1291658,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "931 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5516,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "217906 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2082,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "558962 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5514,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "215732 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5190,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "229711 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 284,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "3977643 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 71.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16842055 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 53.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22283460 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11783876 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.657,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 66.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17915997 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 64.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19087699 times\n2 procs"
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
          "id": "19523be61be52db095e6094a4aa0c2fcb6e207eb",
          "message": "Add Parallel",
          "timestamp": "2020-05-16T00:47:43+02:00",
          "tree_id": "a07ca9dc4ca37060c60143a55419778647137032",
          "url": "https://github.com/Fenny/fiber/commit/19523be61be52db095e6094a4aa0c2fcb6e207eb"
        },
        "date": 1589582952126,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 313,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3813976 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 196,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5794238 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 252,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4625444 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 274,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4269870 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 668,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1820883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14752038 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 161,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7459129 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 292,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4101283 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 240,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5018980 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 680,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1833004 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2314,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "510817 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 734,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1647259 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 215,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5221878 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 71.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16548470 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1010,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 158,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7571626 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 47.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25019790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 85.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14292952 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 407,
            "unit": "ns/op\t     216 B/op\t       4 allocs/op",
            "extra": "2785138 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 60.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20030968 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 998,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1215034 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50347371 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4977,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "236055 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1676098,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "710 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5907,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "201264 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2060,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "584390 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5946,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "200578 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5205,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "231681 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 278,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4315917 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 58.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20570056 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 45.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26063119 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11947736 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.633,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 59.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19910760 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 51.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23400608 times\n2 procs"
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
          "id": "c447684ce874febe6e02ff6418b3d3140c9681a1",
          "message": "Update utils.go",
          "timestamp": "2020-05-16T00:49:14+02:00",
          "tree_id": "e8448f89e1ced9b3f3cc05884fb26377755de778",
          "url": "https://github.com/Fenny/fiber/commit/c447684ce874febe6e02ff6418b3d3140c9681a1"
        },
        "date": 1589583042301,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5092303 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 172,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7599745 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 213,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5739594 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 242,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4987830 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 553,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2281891 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 71.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16555744 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 136,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8650390 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 263,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4746500 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 212,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5164412 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 547,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2258911 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2062,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "642994 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 570,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1988071 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 178,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6144074 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 66.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19253196 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 858,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1372126 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 153,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8293978 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 46,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25732501 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16516048 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 311,
            "unit": "ns/op\t     242 B/op\t       4 allocs/op",
            "extra": "3698560 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 55.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20682372 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 834,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1409196 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50860470 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 3981,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "318914 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1047072,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1170 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4606,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "250632 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1694,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "734167 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4621,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "287307 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "289032 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 233,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "5540728 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20905651 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 42.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25424689 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 86.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14289064 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.508,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 52.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23738160 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 50.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24902202 times\n2 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "ReneWerner87@googlemail.com",
            "name": "ReneWerner87",
            "username": "ReneWerner87"
          },
          "committer": {
            "email": "ReneWerner87@googlemail.com",
            "name": "ReneWerner87",
            "username": "ReneWerner87"
          },
          "distinct": true,
          "id": "2f8c43636d2857b18a330405e8a32fb370e07179",
          "message": "toLower",
          "timestamp": "2020-05-16T00:53:26+02:00",
          "tree_id": "f4c90db5a5456a5eef935564497f021fd42dc60f",
          "url": "https://github.com/Fenny/fiber/commit/2f8c43636d2857b18a330405e8a32fb370e07179"
        },
        "date": 1589583292977,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 284,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4116330 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 189,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6080726 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 246,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4950751 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 263,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4477579 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 644,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1863814 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13531555 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 150,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7727277 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 278,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4284513 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 229,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5087180 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 636,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1886215 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2208,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "541878 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 714,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1734988 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 204,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5689946 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17814140 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1009,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 161,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7174921 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 45.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26070102 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 81,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14358694 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 381,
            "unit": "ns/op\t     209 B/op\t       4 allocs/op",
            "extra": "2903540 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 56.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20230706 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 264,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "4512583 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "53198888 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4993,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "230571 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1586537,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "751 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5618,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "212019 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2127,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "586178 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5622,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "197901 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5028,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "240084 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 274,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4333824 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 59.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20987702 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 46.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26061512 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 89.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13452144 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.616,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 57.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20686362 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 49,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24246141 times\n2 procs"
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
          "id": "14c2c631b9eb0ff72f8c8516db76fdc1d4bd3328",
          "message": "Merge branch 'master' of https://github.com/Fenny/fiber",
          "timestamp": "2020-05-16T00:56:56+02:00",
          "tree_id": "73de3c45c96400284f9322a82ab07546c02517e8",
          "url": "https://github.com/Fenny/fiber/commit/14c2c631b9eb0ff72f8c8516db76fdc1d4bd3328"
        },
        "date": 1589583505373,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 284,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4372430 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 196,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5914459 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 242,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4577202 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 255,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4610841 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 664,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1920824 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 77.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14894043 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 157,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7326626 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 280,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4348554 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 227,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5257690 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 634,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2003919 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2204,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "497238 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 704,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1757812 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 203,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5973920 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17446777 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 972,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1224861 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 149,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8188508 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28759938 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14063392 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 399,
            "unit": "ns/op\t     238 B/op\t       4 allocs/op",
            "extra": "3038391 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 87.2,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12954918 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1013,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54775755 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4974,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "231411 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1587620,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "768 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5758,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "206365 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1966,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "605534 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5616,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "206184 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5057,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223449 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 267,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4500148 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 53.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22906662 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 44.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23639541 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 86.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13404390 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.818,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 87.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "12913276 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 89.2,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13468632 times\n2 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "ReneWerner87@googlemail.com",
            "name": "ReneWerner87",
            "username": "ReneWerner87"
          },
          "committer": {
            "email": "ReneWerner87@googlemail.com",
            "name": "ReneWerner87",
            "username": "ReneWerner87"
          },
          "distinct": true,
          "id": "d10c6421414cda492215743444f35cbe791c3b1d",
          "message": "toLower && toUpper",
          "timestamp": "2020-05-16T01:02:30+02:00",
          "tree_id": "d7765bdf939ef9029a100fdca88e2f9ca8090ac7",
          "url": "https://github.com/Fenny/fiber/commit/d10c6421414cda492215743444f35cbe791c3b1d"
        },
        "date": 1589583848203,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 285,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4339982 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 185,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6213836 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 232,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5139292 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 258,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4561950 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 629,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1879514 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 75.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16185079 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 157,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7456720 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 274,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4211174 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 223,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5148926 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 641,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1947074 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2260,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "518062 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 711,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1703956 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 218,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5267215 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 63,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19682151 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 949,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1239752 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 156,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7614057 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 44.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27973867 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13812627 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 382,
            "unit": "ns/op\t     241 B/op\t       4 allocs/op",
            "extra": "2976190 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 87.8,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13259689 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1023,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1209633 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "48519862 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "239824 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1618544,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "744 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5751,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "202543 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "553972 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5479,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "215314 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4851,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "238651 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 266,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4550128 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22886148 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 50.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22793539 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 87,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13392506 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.805,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 86.9,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13522969 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 86,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14470470 times\n2 procs"
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
          "id": "9ac1ed3fadabc7ddda3decd23f56e58d6e74f432",
          "message": "Merge branch 'master' of https://github.com/Fenny/fiber",
          "timestamp": "2020-05-16T01:04:31+02:00",
          "tree_id": "adf3f0665b44cab7985a035c62d035115c3bac58",
          "url": "https://github.com/Fenny/fiber/commit/9ac1ed3fadabc7ddda3decd23f56e58d6e74f432"
        },
        "date": 1589583971026,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 288,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3970479 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 190,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5871427 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 236,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5049661 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 254,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4299390 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 626,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1917468 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 76.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15454614 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 154,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7586408 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 267,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4547594 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 227,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5363776 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 631,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1909138 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2164,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "551186 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 709,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1712742 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 207,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5043529 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17700662 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 982,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1270819 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 153,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7865725 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29492222 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 80,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14665518 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 369,
            "unit": "ns/op\t     210 B/op\t       4 allocs/op",
            "extra": "2892392 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 93.3,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13391175 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1058,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "52212074 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5172,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "242911 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1596090,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "714 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5622,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "202879 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1941,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "585865 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5793,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "214874 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5143,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "234058 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 265,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "4588108 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 53.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22273458 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 42,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28318792 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 81.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14139104 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.786,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 88.2,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13683087 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 88.6,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "11698796 times\n2 procs"
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
          "id": "015eb7c15f535c493f36f0c03062ceab279548b6",
          "message": "Remove duplicate",
          "timestamp": "2020-05-16T04:45:15+02:00",
          "tree_id": "0754cfe2efb37280d2246dacbfd10ca0391a067b",
          "url": "https://github.com/Fenny/fiber/commit/015eb7c15f535c493f36f0c03062ceab279548b6"
        },
        "date": 1589597200659,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 247,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5092360 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 159,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7249111 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 196,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6049443 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 218,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "5300564 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 519,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2310763 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 66.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "17463858 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 130,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9399616 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 274,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4922356 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 219,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5190416 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 510,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2368683 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1937,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "683416 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 565,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2169771 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 220,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5400000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 62.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19009110 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 854,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1318333 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 150,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "9197322 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 47.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24161535 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 65.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18832798 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 299,
            "unit": "ns/op\t     223 B/op\t       4 allocs/op",
            "extra": "4162203 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 92.6,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14039598 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 923,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "1269826 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 22.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "55458744 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 3967,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "291464 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 990557,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1162 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4226,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "288130 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1639,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "787638 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4136,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "278110 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4109,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "280018 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 222,
            "unit": "ns/op\t     104 B/op\t       3 allocs/op",
            "extra": "5678058 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21123818 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 38.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "33848385 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 95.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10627320 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.551,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toLower",
            "value": 84.5,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "13478766 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_toUpper",
            "value": 81.9,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "14735566 times\n2 procs"
          }
        ]
      }
    ]
  }
}