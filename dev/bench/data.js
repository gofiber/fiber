window.BENCHMARK_DATA = {
  "lastUpdate": 1589572016559,
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
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "e009a7112d6eac5a5b97123967a10c22d1775a6f",
          "message": "Merge pull request #368 from Fenny/master\n\nRemove benchmark results",
          "timestamp": "2020-05-13T01:36:48+02:00",
          "tree_id": "d491eba164028d77c7b3883bf98156d5e380de31",
          "url": "https://github.com/gofiber/fiber/commit/e009a7112d6eac5a5b97123967a10c22d1775a6f"
        },
        "date": 1589326696410,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 291,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4244036 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 190,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6341486 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 237,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4951016 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 262,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4346790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 76.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15730940 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "47319430 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57881407 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16134276 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 155,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6913072 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 611,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1968958 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 147,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7810063 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 287,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4112541 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 233,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5171995 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 658,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1800800 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2168,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "464263 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 531,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2314803 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 701,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1757994 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 203,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5495898 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12208940 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 955,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1223350 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "63866694 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "143062617 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 30.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37460472 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20923296 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 45.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26575016 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 79.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15221502 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 313,
            "unit": "ns/op\t     135 B/op\t       3 allocs/op",
            "extra": "3868903 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 100,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11850890 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 6012,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "201406 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1790273,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "662 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 7024,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "169862 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2336,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "517588 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 7151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "155389 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 6242,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "193005 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 202,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5819608 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 52.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22901061 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.93,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "400329291 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 31.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "37165570 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.07,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "385477770 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 42.1,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27295048 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 128,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9384985 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 46,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26277286 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 85.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13297944 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.387,
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
          "id": "3a0d808152fc0e9e70ab89bfb0ac398fe82f2040",
          "message": "Update ctx_bench_test.go",
          "timestamp": "2020-05-13T01:46:16+02:00",
          "tree_id": "f97a347aaefda428e6ca0835b0d24d03776d0335",
          "url": "https://github.com/Fenny/fiber/commit/3a0d808152fc0e9e70ab89bfb0ac398fe82f2040"
        },
        "date": 1589327273503,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 295,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4120700 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 198,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6144157 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 248,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4799552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 281,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4180045 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15354260 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 12,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 25.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "44775786 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "58786066 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 71.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17130375 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 157,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7711566 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 619,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1973161 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8018668 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 294,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4135304 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 243,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5021515 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 642,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1882988 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2227,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "559392 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 544,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2221252 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 713,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1719486 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 217,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5113296 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10574877 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1015,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65996918 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.53,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "137921449 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "36507739 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21090718 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28195401 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15012502 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 388,
            "unit": "ns/op\t     210 B/op\t       4 allocs/op",
            "extra": "2891808 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11267757 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 6191,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "197670 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1854184,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "667 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 7620,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "159366 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2436,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "490680 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 7320,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "163934 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 6336,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "187002 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 206,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5723662 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 54.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21809421 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.08,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "389075718 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 34.1,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "32592232 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "369772623 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 45.4,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27993544 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 134,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8984247 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 44.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27142381 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 84.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12286449 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.404,
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
          "id": "31f65aa842374badc10d8931223ebcb72c967014",
          "message": "Update app_test.go",
          "timestamp": "2020-05-13T02:54:35+02:00",
          "tree_id": "204b21d93eba887f41820e01bbc2fd7e09aa61cb",
          "url": "https://github.com/Fenny/fiber/commit/31f65aa842374badc10d8931223ebcb72c967014"
        },
        "date": 1589331368474,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 256,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4630452 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 165,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7146205 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 221,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5679026 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 235,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "5092939 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 66.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "18422314 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 10.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 20,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54683004 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 16.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "75062672 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 57.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20708347 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 135,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "9091843 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 532,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2203065 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 127,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9649173 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 243,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4560714 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 202,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "6171445 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 531,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2107298 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1868,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "586176 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 434,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2773688 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 575,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2129778 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 185,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6551415 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 85.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15003220 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 852,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1437334 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 15.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "73734879 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 7.05,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "174879112 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 25.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50007440 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 45.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23527676 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 34.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37567344 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 68.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17413897 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 307,
            "unit": "ns/op\t     217 B/op\t       4 allocs/op",
            "extra": "3455130 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 85.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15246016 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5000,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "222778 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1564780,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "792 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 6039,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "198169 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1960,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "588361 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 6123,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "201050 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5288,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "230859 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 172,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6468718 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 44.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26384914 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.34,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "490955638 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 26.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "45671629 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.63,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "457326537 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 35.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "32148807 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10878229 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 36.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "34259755 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 69.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17679469 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.669,
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
          "id": "eb917359b59faeb4b7ed6b8dcb568d00e5158188",
          "message": "Merge remote-tracking branch 'upstream/master'",
          "timestamp": "2020-05-13T02:59:35+02:00",
          "tree_id": "204b21d93eba887f41820e01bbc2fd7e09aa61cb",
          "url": "https://github.com/Fenny/fiber/commit/eb917359b59faeb4b7ed6b8dcb568d00e5158188"
        },
        "date": 1589331671648,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 303,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3947511 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 209,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5472259 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 257,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4348363 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 292,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4171158 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 87.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "12841465 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 12.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "94492905 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "47581630 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 23.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "52891743 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 78.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14653662 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 180,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6161322 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 641,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1838746 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 156,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7621860 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 313,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3935548 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 285,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4254724 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 660,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1823582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2313,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "480504 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 536,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2196214 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 703,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1720950 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 230,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5492239 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 115,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10655336 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1002,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "63488968 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "137334832 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 33,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "35291654 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 66.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17659594 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 56.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21873432 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12981572 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 361,
            "unit": "ns/op\t     225 B/op\t       4 allocs/op",
            "extra": "3268599 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 107,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11565306 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4873,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "234643 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1256950,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "962 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5439,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223428 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2030,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "601455 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5471,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "222751 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5135,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "225646 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 218,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5496411 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 60.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19088892 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "377185090 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35531710 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.93,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "410949226 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 47.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "24143630 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7299219 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 52.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24306441 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 96.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12757310 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.403,
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
          "id": "28f07a722bd7f86c26d8fb9ddd8bacf1e9d07549",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T03:02:37+02:00",
          "tree_id": "44bbf069499b58c8bdda56975b18a8f3daf9f334",
          "url": "https://github.com/Fenny/fiber/commit/28f07a722bd7f86c26d8fb9ddd8bacf1e9d07549"
        },
        "date": 1589331856620,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 317,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3933484 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 208,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6186171 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 273,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4337678 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 291,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3710554 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 84.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15165426 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "89994952 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50453233 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 22.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "53047972 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 72.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17311639 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 188,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7571102 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 661,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1769881 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 149,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7483801 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 359,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3303933 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 302,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4354953 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 681,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1686830 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2308,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "522687 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 530,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2241534 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 702,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1784004 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 215,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5580661 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 113,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9320487 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1005,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 19.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "68832890 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.48,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "143489828 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40561534 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 71.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18077590 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 63.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18720820 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14002921 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 396,
            "unit": "ns/op\t     242 B/op\t       4 allocs/op",
            "extra": "2958790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 119,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10886485 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5077,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "219351 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1357600,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "867 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5501,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "197624 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2100,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "527218 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5038,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "216841 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5365,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "210884 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 202,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5635921 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 65.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15286266 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.08,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "388050852 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 34.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "34519453 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.06,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "414101156 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 47.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "23832565 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 149,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8188959 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 49.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21653808 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10607661 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.529,
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
          "id": "05de4de055b7eba81a8c0cd673df4e8c8b07ef25",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T03:04:52+02:00",
          "tree_id": "d4a43df7bc34a8e1219c1cb380b1af57ec0bba0b",
          "url": "https://github.com/Fenny/fiber/commit/05de4de055b7eba81a8c0cd673df4e8c8b07ef25"
        },
        "date": 1589331996514,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 316,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3780256 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 211,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5669346 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 270,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4664409 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 314,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3831099 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 87.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14189378 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 12,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "95760009 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 25,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "46943485 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 21.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "56000952 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 77.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15710667 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 185,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6320695 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 676,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1794146 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7619229 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 364,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3565140 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 283,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4279963 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 715,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1621135 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2290,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "478365 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 572,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2193139 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 780,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1639450 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 228,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5200060 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 119,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10012092 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1053,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 19.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "58406594 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.78,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "137939876 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 34.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "33867495 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 71.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15145692 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 60.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18860289 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 88.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12994762 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 370,
            "unit": "ns/op\t     229 B/op\t       4 allocs/op",
            "extra": "3207963 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 115,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9777638 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4879,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "250962 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1272056,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "934 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5748,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "222001 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2191,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "506920 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5717,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "213997 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5304,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "241036 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 230,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5222258 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 74.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16073634 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.34,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "383228482 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 35.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "34584182 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "371395245 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 50.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "24413468 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 166,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6875206 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 58.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19938876 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 107,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9846318 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.493,
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
          "id": "8f82bee55af82d939d52e21b63c3931c6addfb2e",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T03:07:28+02:00",
          "tree_id": "44bbf069499b58c8bdda56975b18a8f3daf9f334",
          "url": "https://github.com/Fenny/fiber/commit/8f82bee55af82d939d52e21b63c3931c6addfb2e"
        },
        "date": 1589332165628,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 329,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3668836 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 213,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5093764 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 286,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4091241 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 318,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3681950 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 95,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "12889486 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 13.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100718991 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 27.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41816904 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 24.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "48658837 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 82.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15723442 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 196,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5987463 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 703,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1711695 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 166,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6505064 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 376,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3490183 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 308,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3845746 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 768,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1538401 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2454,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "526321 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 582,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2058619 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 765,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1609998 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 238,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5316661 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 129,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9320530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1145,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 21.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "53501954 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.89,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "126863206 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 35.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "35922086 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 69.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15562365 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 56.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18077710 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 91.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12384142 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 446,
            "unit": "ns/op\t     228 B/op\t       4 allocs/op",
            "extra": "2579970 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 120,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10977597 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5358,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "224234 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1387165,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "975 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 6131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "191318 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2145,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "512776 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5887,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "194652 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5691,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "179720 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 217,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5077017 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 64.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19008682 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "384436718 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 37.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "30057998 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "409560417 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 53.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "22450760 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 166,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "6725101 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 63,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19195087 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10164229 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.527,
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
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "f5ad2c07666ca4569da89a80df075c4c159cc891",
          "message": "Merge pull request #370 from Fenny/master\n\nUpdate tests",
          "timestamp": "2020-05-13T04:10:42+02:00",
          "tree_id": "44bbf069499b58c8bdda56975b18a8f3daf9f334",
          "url": "https://github.com/Fenny/fiber/commit/f5ad2c07666ca4569da89a80df075c4c159cc891"
        },
        "date": 1589375122423,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 297,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4017020 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 201,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5940835 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 253,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4742593 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 277,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3963469 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15011762 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "47411948 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "58570358 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 74.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16180830 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 168,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7184809 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 631,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1903946 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7821009 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 321,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3774074 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 270,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4519041 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 641,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1843815 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2241,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "464176 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 541,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2242422 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 689,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1706919 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 213,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5784307 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 116,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10411892 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 976,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1232402 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "66708589 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.45,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "140811530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38672400 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16239224 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 55.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21439533 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14397492 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 343,
            "unit": "ns/op\t     221 B/op\t       4 allocs/op",
            "extra": "3357139 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11431332 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4728,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "242216 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1242064,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "974 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5380,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "224820 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2030,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "523836 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5388,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220833 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4981,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "236055 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 205,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5874356 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 67.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18438627 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.04,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "387930148 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "36698388 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.96,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "406861239 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 46.4,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25798974 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7986633 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 57.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20832439 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11888774 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.545,
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
          "id": "ca7dbb5e4c3b8a1a1bda89f5503e4917488081a8",
          "message": "Add tests & benchmark",
          "timestamp": "2020-05-13T15:22:58+02:00",
          "tree_id": "fa6b7e2c9047f4ec0b075bb349a398fc76982586",
          "url": "https://github.com/Fenny/fiber/commit/ca7dbb5e4c3b8a1a1bda89f5503e4917488081a8"
        },
        "date": 1589376285996,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 306,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3979126 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 198,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6305211 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 246,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4742061 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 268,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4377530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13179792 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 12.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "48773829 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "66254094 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17430076 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 161,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7699864 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 627,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1894438 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 153,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7989391 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 294,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4019163 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 238,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4993506 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 641,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1857801 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2347,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "495314 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 534,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2246444 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 706,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1678489 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 209,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5590798 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11339726 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 986,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1204671 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54998458 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.58,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "139348588 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38851336 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21125388 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26218336 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 82.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13616071 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 363,
            "unit": "ns/op\t     210 B/op\t       4 allocs/op",
            "extra": "2898702 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11894488 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 183,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6553665 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 75.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14996425 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1589,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "721902 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38205657 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 6179,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "197878 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1816212,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "661 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 7279,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "146878 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2517,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "478045 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 7270,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "165630 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 6515,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "174918 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 209,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5823177 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 53.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22497176 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.09,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "391180911 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35921406 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.17,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "381107389 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 44.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25692813 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8831583 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 43.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26839603 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 89.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13636628 times\n2 procs"
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
          "id": "5f226c9b7c79d6195569df71a76fbbeba5e6c2c6",
          "message": "Update app_test.go",
          "timestamp": "2020-05-13T16:01:47+02:00",
          "tree_id": "ab3f8ebbfa4219243478a9e160c22774e9d24434",
          "url": "https://github.com/Fenny/fiber/commit/5f226c9b7c79d6195569df71a76fbbeba5e6c2c6"
        },
        "date": 1589378621975,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 271,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4150418 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 182,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6422538 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 237,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5057223 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 258,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4528675 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 78.1,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15343910 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 22.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "56190787 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57793384 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 68.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17375902 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 151,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7629283 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 577,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2042137 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 144,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8592015 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 308,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3961538 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 254,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4728932 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 623,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1954250 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2209,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "560660 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 508,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2288560 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 642,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1746076 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 205,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5420115 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11076796 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 912,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1339106 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 17.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "71043704 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "145142143 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 29.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42154689 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 58.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20518101 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 49.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21667428 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 79.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15340286 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 339,
            "unit": "ns/op\t     217 B/op\t       4 allocs/op",
            "extra": "3454335 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 95.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12118837 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 185,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6335094 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 69,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15959360 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1112,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "35442433 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4318,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "287400 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1143943,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1015 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4864,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "243300 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1852,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "696128 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "252420 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4586,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "258068 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 194,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6241624 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19318411 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.88,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "421179408 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 31.1,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "38497216 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.66,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "448115532 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "24926042 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 141,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8555650 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 51.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22400079 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 83.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14008906 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.367,
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
          "id": "a032d23a665612536da1de9888dc506f2e3a38b6",
          "message": "Update app_test.go",
          "timestamp": "2020-05-13T16:38:24+02:00",
          "tree_id": "bfb7489f4fda5f0170bc9a09702e2dfc561739a4",
          "url": "https://github.com/Fenny/fiber/commit/a032d23a665612536da1de9888dc506f2e3a38b6"
        },
        "date": 1589380813327,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 301,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4031319 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 198,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5989046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 244,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4948953 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 271,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4364390 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 78.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15374949 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "94385367 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "44542155 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "66916330 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17623383 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 154,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7773474 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 622,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1918087 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 149,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8062719 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 291,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3900589 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 231,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5107364 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 631,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1913275 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2318,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "525975 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 540,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2199630 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 696,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1699396 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 209,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5205001 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11161135 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 973,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1231473 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65914486 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "139890693 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37919946 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21105379 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 42.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25543213 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 82.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15125419 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 394,
            "unit": "ns/op\t     240 B/op\t       4 allocs/op",
            "extra": "2994271 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11834361 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 178,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6511574 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 74.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16706979 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1586,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "740816 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42055689 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 6019,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "192418 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1801901,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "652 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 7346,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "161166 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2444,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "479305 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 7358,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "170394 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 6458,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "190142 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 207,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5682066 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 56.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20470908 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.08,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "397893236 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 34.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35570540 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "373681226 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 45.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25578555 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8959467 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 46.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26331915 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 90.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12180804 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.424,
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
          "id": "4dcbd375121eaa9bce5b540a268ff68d849c5a07",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T17:51:40+02:00",
          "tree_id": "2a21cbf1e235628ae04ffdb41b27a752d8ea1b16",
          "url": "https://github.com/Fenny/fiber/commit/4dcbd375121eaa9bce5b540a268ff68d849c5a07"
        },
        "date": 1589385207131,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 283,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4267548 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 191,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6127849 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4902891 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 269,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4559134 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 84.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14560374 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "56081492 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 19.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "61269153 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 70,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18013981 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 162,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7999354 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 620,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2049919 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8209042 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 318,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3958800 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 271,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4616706 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 659,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1804360 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2388,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "534456 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 531,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2222786 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 680,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1678599 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 207,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5699600 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11849794 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 968,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1271401 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 17.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "62523706 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 7.79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "139858123 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 29.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "43148101 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 58.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19581253 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 52.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24540355 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 78.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15880590 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 354,
            "unit": "ns/op\t     239 B/op\t       4 allocs/op",
            "extra": "3008570 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11643633 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 201,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6321144 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 71.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16134560 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1170,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "960950 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 31.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37806988 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4475,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "280291 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1225998,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1033 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5201,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "233626 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1972,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "598752 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4980,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "248226 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4800,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "254083 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 206,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5992364 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 63.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18779872 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "405514273 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "38544832 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "427486916 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 46.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26228445 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 144,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8012794 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 44.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25911678 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11936263 times\n2 procs"
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
          "id": "4da7cca4fff0e7c71e42ad724ccd403882ac41b4",
          "message": "New: app.Add",
          "timestamp": "2020-05-13T18:51:58+02:00",
          "tree_id": "128b21367577340bdf3a231372c7ac56a5d72e15",
          "url": "https://github.com/Fenny/fiber/commit/4da7cca4fff0e7c71e42ad724ccd403882ac41b4"
        },
        "date": 1589388825996,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 271,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4528424 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 179,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6558499 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 238,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5065261 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 257,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4569348 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 74.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15189818 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49686099 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 16.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "69699930 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 61.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17442064 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 143,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8217882 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 601,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1996872 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 139,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8291734 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 280,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4485235 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 227,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5420552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 629,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1856850 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2234,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "522021 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 508,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2318332 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 664,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1750051 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 194,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6313185 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 99.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12324667 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 931,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1279610 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 17.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "68920472 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "153035988 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41480266 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 54.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21161930 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 43.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28903015 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 78.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15109352 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 379,
            "unit": "ns/op\t     235 B/op\t       4 allocs/op",
            "extra": "3084039 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 100,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11896573 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 176,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6821474 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 69.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "17497215 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1064,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "43243308 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4656,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "253119 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1407890,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "870 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5621,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220579 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1921,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "642015 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5540,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "221674 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4906,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "251928 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 197,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5760966 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 54.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22278616 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.95,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "402732673 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 31.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35730058 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.03,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "392275215 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27851394 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9367485 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 41.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26506837 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 89,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13144202 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.397,
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
          "id": "f0e16aed71f5d5c1ed4f54c01a2bc9cd6cb109e3",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T19:41:39+02:00",
          "tree_id": "527ffde38de2a224ecafa28bdc85f9e32b131366",
          "url": "https://github.com/Fenny/fiber/commit/f0e16aed71f5d5c1ed4f54c01a2bc9cd6cb109e3"
        },
        "date": 1589391801311,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 312,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3853136 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 201,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5956005 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 249,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4771485 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 275,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4403422 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.1,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13440712 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 12.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "98783750 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 25.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "45970317 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 21.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "56677620 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 70,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17399716 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 159,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7167132 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 637,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1830806 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7736899 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 294,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4039322 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 245,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4882981 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 688,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1749847 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2303,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "512151 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 546,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2191892 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 733,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1660071 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 216,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5585641 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10979506 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1034,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 19,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "64407273 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "134721362 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 33,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "36635899 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 60.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20316153 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 46.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26258796 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14191605 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 398,
            "unit": "ns/op\t     211 B/op\t       4 allocs/op",
            "extra": "2878882 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11154992 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 188,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6320244 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 74.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15964983 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1173,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 30.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38934699 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4985,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "217101 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1510626,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "757 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 6025,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "200430 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2120,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "579093 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 6069,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "202308 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5277,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "229900 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 215,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5779834 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 57.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20638112 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.09,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "388300820 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "33991707 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.31,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "366657681 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 44.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "24678644 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 137,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8918916 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 51.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23487624 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 91.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12886908 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.432,
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
          "id": "0c0ff3d1fe5c811da8c2f88a025321e16cec0964",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T19:50:46+02:00",
          "tree_id": "201728fcd45282736d9e917ed3a38873e98e887a",
          "url": "https://github.com/Fenny/fiber/commit/0c0ff3d1fe5c811da8c2f88a025321e16cec0964"
        },
        "date": 1589392411971,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 272,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4227694 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 181,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6072630 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 234,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5076766 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 266,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4279262 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 73.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16242192 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 10.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 21.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "55672278 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57895464 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 65.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15501303 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 154,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7958953 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 596,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1958835 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 132,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8974796 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 280,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4029260 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 231,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4595778 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 658,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1733034 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2420,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "529468 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 514,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2454564 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 637,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1850606 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 197,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5719455 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 98.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11803410 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1167,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 16.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "77817957 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 7.66,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "159839888 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31490727 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 61.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19595941 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 48.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23014338 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 73,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16309924 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 451,
            "unit": "ns/op\t     234 B/op\t       4 allocs/op",
            "extra": "2487306 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 95.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12116505 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 182,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6801544 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 69.5,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "18867423 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1089,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "36871917 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4166,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "272452 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1120553,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1146 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4681,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "259051 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1977,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "632115 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4854,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "229977 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4645,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "290383 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 201,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5619564 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 59.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20627936 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.44,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "467353993 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 29.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "41684149 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.75,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "470208552 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 41.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "28168890 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 148,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8270083 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 50.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20658506 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 94.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12450165 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.344,
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
          "id": "2c562f34663430db20e4cbb607bfd105a47140d9",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T19:54:51+02:00",
          "tree_id": "128b21367577340bdf3a231372c7ac56a5d72e15",
          "url": "https://github.com/Fenny/fiber/commit/2c562f34663430db20e4cbb607bfd105a47140d9"
        },
        "date": 1589392601106,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 275,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4200552 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 185,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6444775 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 234,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4829077 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 256,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4566090 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 76,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16305748 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 10.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49929874 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "63324421 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16327870 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 157,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7266200 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 636,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2048541 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 145,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8391734 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 297,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4093690 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 248,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4705860 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 688,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1750434 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2405,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "493940 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 499,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2415085 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 664,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1831261 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 199,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5742711 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10887349 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1231,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 17.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "70719890 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.04,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "151193308 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 29.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42104110 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 59.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19647244 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 51.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24564075 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 77.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15747846 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 460,
            "unit": "ns/op\t     228 B/op\t       4 allocs/op",
            "extra": "2569879 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 98.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12327361 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 185,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6478329 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 68.7,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15699744 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1099,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 30.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39250879 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4402,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "268266 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1163668,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1041 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5026,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "243786 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1911,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "628879 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4977,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "241672 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4759,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "235369 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 196,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6178581 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 62.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19354213 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.63,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "462745334 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 31,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "38875387 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.72,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "448381252 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26739598 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 144,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8458400 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 47.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25597344 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 91.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13191913 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.361,
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
          "id": "66c8f43f212c5a28e5480c3548db3769d2d7a395",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T20:07:20+02:00",
          "tree_id": "55d04342e9520bf439c5912a31703af1e481237b",
          "url": "https://github.com/Fenny/fiber/commit/66c8f43f212c5a28e5480c3548db3769d2d7a395"
        },
        "date": 1589393345157,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 301,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3985839 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 195,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6080017 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 242,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5152864 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 263,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4603111 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 77.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "14504725 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49504740 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "64134415 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 66.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17029766 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 154,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6742980 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 624,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1938288 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7703400 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 286,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4097926 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 232,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4983927 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 694,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1825725 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2273,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "511714 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 537,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2296243 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 697,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1681192 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 217,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5735326 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11452450 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1007,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "64681836 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.56,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "136339150 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 32.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31311058 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 59.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20599838 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 45.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24268870 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14731957 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 403,
            "unit": "ns/op\t     219 B/op\t       4 allocs/op",
            "extra": "2730621 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11816398 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 181,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6710073 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 73.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16110607 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1110,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 31.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39224343 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4889,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "240643 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1476331,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "796 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5708,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "193958 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "581226 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5767,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "208118 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5127,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "238837 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 206,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5855089 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 58.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20946396 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.99,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "400608884 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "36180226 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.15,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "376231492 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 44.4,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26592822 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 134,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9001878 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 42.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27972759 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 88.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13689369 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.405,
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
          "id": "4969e9ef69ac61b36ce547417635bbcace7db1ee",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T21:25:20+02:00",
          "tree_id": "55d04342e9520bf439c5912a31703af1e481237b",
          "url": "https://github.com/Fenny/fiber/commit/4969e9ef69ac61b36ce547417635bbcace7db1ee"
        },
        "date": 1589398019271,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 255,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4613677 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 165,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7051633 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 205,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5680658 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 235,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "5014766 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 69.1,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "18202827 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 10.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "54090660 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 15.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "74345826 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 60.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18527083 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 143,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8483935 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 546,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2175945 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 129,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9436579 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 246,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4859988 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 203,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5594478 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 578,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "2042720 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 1990,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "580909 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 457,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2608386 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 605,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1905514 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 193,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5795793 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 90.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13207762 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 828,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1430152 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 15.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "77818336 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 7.33,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "155596558 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 28.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40193070 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 48.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22709496 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 39.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "31610216 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 69,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16967766 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 330,
            "unit": "ns/op\t     215 B/op\t       4 allocs/op",
            "extra": "3504687 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 92.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11884186 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 160,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "7871235 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 63.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "19466412 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 949,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1249863 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 25.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "45720718 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4096,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "288859 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1269108,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "970 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4891,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "243568 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1725,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "671071 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4866,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "266323 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4274,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "288810 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 177,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6776348 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 50.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23281590 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "457328774 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 27.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "41204961 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.79,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "434542219 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 40.1,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26863600 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 119,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10079846 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 38.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "28771854 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 72.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16154689 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.339,
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
          "id": "a6cbd3ecf15210613340bf8866badac41240033a",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T23:08:24+02:00",
          "tree_id": "fcc2afe7b0583bc2024c20c767724fe9884dec7e",
          "url": "https://github.com/Fenny/fiber/commit/a6cbd3ecf15210613340bf8866badac41240033a"
        },
        "date": 1589410646314,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 280,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4452582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 197,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5985230 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 242,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4928652 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 271,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3948607 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 81.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15337526 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "51770017 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 19.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "63095121 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 69.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16583316 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 160,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7700590 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 592,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1996848 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 146,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8560411 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 323,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3731359 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 270,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4404301 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 735,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1609128 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2463,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "469124 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 520,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2201088 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 659,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1821609 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 210,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5975209 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 107,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10367434 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1355,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "897118 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "69343870 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.39,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "144824300 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42070575 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 61.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20042421 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 52,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23047639 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 83.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14559242 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 475,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "2440964 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11140412 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 197,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6114465 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 75.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15952180 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1200,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 32.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "35505271 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4631,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "258637 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1213060,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "946 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5277,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220366 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2013,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "621279 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5244,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "222606 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4999,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "255795 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 207,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5707994 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 66.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18028962 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.68,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "443833794 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "38069642 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.82,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "418225563 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 45.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25278636 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8001474 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 50,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22317348 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 100,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12532254 times\n2 procs"
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
          "id": "a6cbd3ecf15210613340bf8866badac41240033a",
          "message": "Update benchmark.yml",
          "timestamp": "2020-05-13T23:08:24+02:00",
          "tree_id": "fcc2afe7b0583bc2024c20c767724fe9884dec7e",
          "url": "https://github.com/Fenny/fiber/commit/a6cbd3ecf15210613340bf8866badac41240033a"
        },
        "date": 1589420516096,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 296,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3999807 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 190,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6190712 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 241,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4990059 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 261,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4224672 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 80.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15847803 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49537402 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "57763081 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17602160 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 151,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7930743 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 620,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1972395 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 152,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8044152 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 280,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4171477 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 240,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5011153 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 660,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1788674 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2383,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "495932 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 525,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2208798 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 708,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1721653 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 205,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5915860 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11544847 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 964,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1218616 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65964343 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.51,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "141242514 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37988130 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21065817 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 44.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26143530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 81.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14386575 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 393,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "3059246 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 102,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11667847 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 177,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6568857 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 71.6,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16993237 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1133,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39225038 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4966,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "228493 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1459023,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "813 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5828,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "209913 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2056,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "525786 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5820,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "208009 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5063,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "234912 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 201,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6010459 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21916524 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "406979182 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "34777956 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.21,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "383134320 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27339214 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9309742 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 45.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25940169 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 90.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13165884 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.408,
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
          "id": "564928ff269c646c37eb7fe9703aedbea39a7b51",
          "message": "v1.9.7",
          "timestamp": "2020-05-15T08:02:49+02:00",
          "tree_id": "390d7d949a71a63b7d79970055f607dcb9dcebb7",
          "url": "https://github.com/Fenny/fiber/commit/564928ff269c646c37eb7fe9703aedbea39a7b51"
        },
        "date": 1589522664402,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 302,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3911846 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 200,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6125019 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4726198 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 267,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4303494 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 613,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1946238 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7783893 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 283,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4269997 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 230,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5090846 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 664,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1842604 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2254,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "499790 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 693,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1769197 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 209,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5600726 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 67.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17105769 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 965,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1229671 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 158,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7262206 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 44.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25761867 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 81.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14840454 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 386,
            "unit": "ns/op\t     243 B/op\t       4 allocs/op",
            "extra": "2956077 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 176,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6490950 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 71.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "17554846 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1043,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 24.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49852005 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4672,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "257121 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1391303,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "877 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5390,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "214687 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1877,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "661564 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5385,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223036 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4872,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "251071 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 195,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6158368 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 55.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21505621 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.84,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "418021682 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 32.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "34759935 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.09,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "387022867 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "26100582 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 125,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9598282 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 42.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27705576 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 85.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13768426 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.382,
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
          "id": "3784f69563ffdcc21c4e062ced409b9c5fc60a4a",
          "message": "Improve strictrouting",
          "timestamp": "2020-05-15T21:22:02+02:00",
          "tree_id": "78aaaea88a79eccaadc59c0aefa44d5274476003",
          "url": "https://github.com/Fenny/fiber/commit/3784f69563ffdcc21c4e062ced409b9c5fc60a4a"
        },
        "date": 1589570607196,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 252,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4965879 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 169,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7050663 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 209,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5324215 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 238,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "5148373 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 545,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2180546 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 132,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9444411 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 314,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3831582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 272,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4433637 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 670,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1696892 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2371,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "563036 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 595,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1974387 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 183,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6678842 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 64.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18986655 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1029,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 145,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8139883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 46.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27028798 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 77.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14342883 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 382,
            "unit": "ns/op\t     239 B/op\t       4 allocs/op",
            "extra": "3025522 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 37.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "32930761 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 838,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1442702 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 23.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "51378229 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4007,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "258189 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1072424,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1112 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 4735,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "247790 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1762,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "685232 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 4557,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "269360 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4322,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "254781 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 182,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6705892 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 52.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21698095 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "485090016 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 29.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "41996520 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.47,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "472928640 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 41.8,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27626220 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 132,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9273126 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 46.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26842456 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 85,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13221596 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.329,
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
          "id": "8bbb3a1a02c7244bd405527a2c53c91ae4d58e3c",
          "message": "Update incase sensitive",
          "timestamp": "2020-05-15T21:45:27+02:00",
          "tree_id": "9b4cc87ed9d92afca5cbff9983f67e674b27a441",
          "url": "https://github.com/Fenny/fiber/commit/8bbb3a1a02c7244bd405527a2c53c91ae4d58e3c"
        },
        "date": 1589572015587,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 304,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3964227 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 203,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5638935 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 254,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4744143 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 280,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "3970557 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 638,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1865198 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7403325 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 386,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3136870 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 289,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4075334 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 767,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1576482 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2556,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "493027 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 721,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1649564 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 231,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5278582 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 71.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "16475025 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1158,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 172,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6870247 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 57,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21660565 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 87.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13958257 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 453,
            "unit": "ns/op\t     237 B/op\t       4 allocs/op",
            "extra": "2445880 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 65.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17860910 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 295,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "4315186 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 28.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "42741759 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5238,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "229710 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1269473,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "906 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5424,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "223414 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2016,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "605758 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5501,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "218647 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5072,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "235750 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 222,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5475463 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 65.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17078798 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.91,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "427326238 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 35.1,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "33152398 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.41,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "351520664 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 49.7,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "24167288 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 155,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7936734 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 48.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23697296 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11118813 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.402,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      }
    ]
  }
}