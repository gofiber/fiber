window.BENCHMARK_DATA = {
  "lastUpdate": 1589331672571,
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
      }
    ]
  }
}