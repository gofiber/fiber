window.BENCHMARK_DATA = {
  "lastUpdate": 1589590191752,
  "repoUrl": "https://github.com/gofiber/fiber",
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
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "e0b13d9ca4b364d4c57ae5c8912b2a9ec6a108b1",
          "message": "Update test & benchmark (#374)\n\nCo-Authored-By: RW <renewerner87@googlemail.com>\r\n\r\n* Add nosec for WriteByte\r\n\r\n* test persistens for benchmark results\r\n\r\n* Add tests & benchmark\r\n\r\n* Update app_test.go\r\n\r\n* Update benchmark.yml",
          "timestamp": "2020-05-13T20:21:49+02:00",
          "tree_id": "55d04342e9520bf439c5912a31703af1e481237b",
          "url": "https://github.com/gofiber/fiber/commit/e0b13d9ca4b364d4c57ae5c8912b2a9ec6a108b1"
        },
        "date": 1589394210328,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 329,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "3659122 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 214,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "5707875 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 261,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4551640 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 301,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4000233 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 87.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "13720028 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 13,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "98929833 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 26.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "45791530 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 22.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "52267503 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 81.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15252087 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 185,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6324778 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 683,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1682004 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 163,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7575670 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 349,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3623974 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 290,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "3899432 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 809,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1474260 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2747,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "419329 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 591,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1957747 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 746,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1610649 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 233,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "4865187 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 118,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9825172 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1448,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "802278 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 20.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "61065572 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "129734046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 33.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "37850340 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 68.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18124718 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 58.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20198851 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 89,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13578834 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 548,
            "unit": "ns/op\t     220 B/op\t       4 allocs/op",
            "extra": "2163373 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 116,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10745415 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 220,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5562526 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 79.3,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15565005 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1253,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "897162 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 34.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "33620689 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 5131,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "241749 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1366338,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "883 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5943,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "197936 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2231,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "547761 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 6095,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "200692 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5544,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "217089 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 235,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5046754 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 65,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17881513 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 3.03,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "396898836 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 39.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "30008697 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.07,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "391169806 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 50.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "23064721 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 158,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7301112 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 52.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22563810 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 106,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11370170 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.425,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "hendra.tommy.w@gmail.com",
            "name": "hendratommy",
            "username": "hendratommy"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "518a1d10aef7e82b03763322e810e7442c130e48",
          "message": "Create .gitignore (#376)\n\nadd .gitignore",
          "timestamp": "2020-05-13T20:30:33+02:00",
          "tree_id": "4758a16d7118b69ece18116bc98c096e768d5f03",
          "url": "https://github.com/gofiber/fiber/commit/518a1d10aef7e82b03763322e810e7442c130e48"
        },
        "date": 1589394733776,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 289,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4158367 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 184,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6332839 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 234,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5360706 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 248,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4986967 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 74.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16553007 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "98849588 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 23.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50647603 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 19.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "60352843 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 63.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "18808036 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 148,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8108793 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 605,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1973474 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 144,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8353675 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 272,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4396942 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 225,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5293359 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 630,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1884637 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2296,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "541599 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 508,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2364441 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 692,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1755459 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 201,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5918764 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10950639 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 931,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1277660 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 17.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "68795982 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.14,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "146130970 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 30.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40223293 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 53.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21599757 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 40.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29434344 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 75.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15566065 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 376,
            "unit": "ns/op\t     236 B/op\t       4 allocs/op",
            "extra": "3066487 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 95.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12125905 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 171,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "7151028 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 66.9,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16689962 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1063,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 27.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "41039928 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4508,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "260265 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1390062,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "883 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5221,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "212178 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1848,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "661221 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5243,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "220233 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4653,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "261738 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 188,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6251566 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 51,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23542038 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.73,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "452653461 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 30.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35490171 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.97,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "414411010 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 41.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "30455044 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 120,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9940328 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 41.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27638438 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 76.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15154390 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.381,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "hendra.tommy.w@gmail.com",
            "name": "hendratommy",
            "username": "hendratommy"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "d6d73f42d01fb44408495843ee90ab396bc7d4bc",
          "message": "Update app_test.go (#377)\n\ntestStatus200 for PUT mistaken with CONNECT",
          "timestamp": "2020-05-13T20:30:59+02:00",
          "tree_id": "89a0b6430d564be0595c7b9f2df7494ab20ddd6d",
          "url": "https://github.com/gofiber/fiber/commit/d6d73f42d01fb44408495843ee90ab396bc7d4bc"
        },
        "date": 1589394760803,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 305,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4003426 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 194,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6030034 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 245,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4880935 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 268,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4331655 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.2,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15766833 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 25.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "49276926 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65947603 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 69.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17343618 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 156,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7915264 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 611,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1948341 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 151,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7990962 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 286,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4258674 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 231,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5252701 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 676,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1767866 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2390,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "469135 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 526,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2241777 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 697,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1683492 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 211,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5437327 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 104,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11773402 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 1009,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 18.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "66787566 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.71,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "141062436 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "34127665 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "21024648 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 45.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26866686 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 84.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14325432 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 398,
            "unit": "ns/op\t     240 B/op\t       4 allocs/op",
            "extra": "2999326 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 105,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11001151 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 183,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "6694362 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 73,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16106596 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1119,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "934814 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 29.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39542091 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4847,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "250813 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1465149,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "826 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5894,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "211964 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 2015,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "590415 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5780,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "206623 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 5022,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "226854 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 205,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5812698 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 58.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20765566 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.98,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "392199630 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 33.3,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "35764831 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.28,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "377728204 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "25229877 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 132,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8973002 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 43.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "26885038 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 94.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "13213152 times\n2 procs"
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
            "email": "abajall@gmail.com",
            "name": "abdulaziz alfuhigi",
            "username": "alfuhigi"
          },
          "committer": {
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "b5f128dd94acb1f5053ec0ecb2d4c949e6e529b7",
          "message": "create README_ar_SA.md (#380)\n\ntranslate README.md to arabic",
          "timestamp": "2020-05-16T02:47:31+02:00",
          "tree_id": "5d8513f0e70c294dabfff7c6fce2ea1c1379c490",
          "url": "https://github.com/gofiber/fiber/commit/b5f128dd94acb1f5053ec0ecb2d4c949e6e529b7"
        },
        "date": 1589590152386,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 294,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4075353 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 192,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "6004124 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 257,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4882352 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 261,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4466648 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 79.8,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "15673916 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 11.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 24.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "50255305 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 20.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "60272930 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 66.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "17126192 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 153,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "7966664 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 615,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "1933365 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 150,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "7615978 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 282,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4269346 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 234,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5220949 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 652,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1799169 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2322,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "543716 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 522,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2269778 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 694,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1711046 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 204,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "5783158 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 103,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11384396 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 957,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1247349 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 19.8,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65003814 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 8.37,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "141805959 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 31.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "38523886 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 57.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "19838949 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 43,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "27658086 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 80.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14852872 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 383,
            "unit": "ns/op\t     229 B/op\t       4 allocs/op",
            "extra": "3206115 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 101,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "11903121 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 176,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "7007011 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 73.5,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "16978651 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1135,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 30,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "39923498 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4718,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "249907 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1420737,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "788 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5738,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "207889 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1966,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "616644 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5779,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "207852 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4975,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "244994 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 200,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "5911026 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 54.5,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "22459400 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.91,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "405902234 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 31.9,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "36307135 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 3.16,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "370813077 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 43.2,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "27926377 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 126,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "9325142 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 45.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "24653155 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 86.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "14263286 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.407,
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
          "id": "e45205fb614862f496d78eb714bb4382529a5931",
          "message": "Update README_ar_SA.md",
          "timestamp": "2020-05-16T02:48:14+02:00",
          "tree_id": "861dbbad65c2d85c72c6723d0cb719ad6c4df636",
          "url": "https://github.com/gofiber/fiber/commit/e45205fb614862f496d78eb714bb4382529a5931"
        },
        "date": 1589590190830,
        "tool": "go",
        "benches": [
          {
            "name": "Benchmark_Ctx_Accepts",
            "value": 274,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "4514818 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsCharsets",
            "value": 178,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "7248753 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsEncodings",
            "value": 216,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "5466085 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_AcceptsLanguages",
            "value": 240,
            "unit": "ns/op\t      80 B/op\t       1 allocs/op",
            "extra": "4803291 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_BaseURL",
            "value": 69,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "17715937 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Body",
            "value": 10.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "100000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookies",
            "value": 22.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "48638198 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_FormFile",
            "value": 18.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "64106854 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Params",
            "value": 60.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "20334351 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Subdomains",
            "value": 142,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "8577757 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Append",
            "value": 583,
            "unit": "ns/op\t      40 B/op\t       5 allocs/op",
            "extra": "2042748 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Cookie",
            "value": 139,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "8439522 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format",
            "value": 273,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "4508503 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_HTML",
            "value": 227,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "5287096 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_JSON",
            "value": 622,
            "unit": "ns/op\t      48 B/op\t       3 allocs/op",
            "extra": "1925457 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Format_XML",
            "value": 2269,
            "unit": "ns/op\t    4480 B/op\t       8 allocs/op",
            "extra": "538581 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSON",
            "value": 501,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "2499776 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_JSONP",
            "value": 622,
            "unit": "ns/op\t      80 B/op\t       3 allocs/op",
            "extra": "1916340 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Links",
            "value": 191,
            "unit": "ns/op\t     112 B/op\t       1 allocs/op",
            "extra": "6075925 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Redirect",
            "value": 95.4,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12746935 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Send",
            "value": 872,
            "unit": "ns/op\t     136 B/op\t      12 allocs/op",
            "extra": "1259560 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendBytes",
            "value": 16.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "65252577 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendStatus",
            "value": 7.74,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "158829129 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_SendString",
            "value": 29.9,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40826680 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Set",
            "value": 53.2,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "23064087 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Type",
            "value": 39.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29944026 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Vary",
            "value": 74.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15816669 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_Write",
            "value": 359,
            "unit": "ns/op\t     230 B/op\t       4 allocs/op",
            "extra": "3182383 times\n2 procs"
          },
          {
            "name": "Benchmark_Ctx_XHR",
            "value": 92.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "12377869 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_CaseSensitive",
            "value": 163,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "7246354 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_StrictRouting",
            "value": 65.4,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "18178346 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Handler",
            "value": 1058,
            "unit": "ns/op\t      32 B/op\t       1 allocs/op",
            "extra": "1000000 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_NextRoute",
            "value": 27.3,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "40481793 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Next_Stack",
            "value": 4263,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "295845 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Github_API",
            "value": 1296130,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "956 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Stacked_Route",
            "value": 5134,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "247398 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Last_Route",
            "value": 1828,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "655832 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_Middle_Route",
            "value": 5502,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "227068 times\n2 procs"
          },
          {
            "name": "Benchmark_Router_First_Route",
            "value": 4570,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "254898 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getGroupPath",
            "value": 184,
            "unit": "ns/op\t      64 B/op\t       1 allocs/op",
            "extra": "6418371 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getMIME",
            "value": 50.7,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "25203568 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getString",
            "value": 2.67,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "433442512 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getStringImmutable",
            "value": 29.6,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "39505966 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytes",
            "value": 2.87,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "427815634 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getBytesImmutable",
            "value": 39.5,
            "unit": "ns/op\t      16 B/op\t       1 allocs/op",
            "extra": "30193735 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_methodINT",
            "value": 117,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "10036984 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_statusMessage",
            "value": 38.1,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "29068383 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_extensionMIME",
            "value": 77.6,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "15506574 times\n2 procs"
          },
          {
            "name": "Benchmark_Utils_getTrimmedParam",
            "value": 0.371,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "1000000000 times\n2 procs"
          }
        ]
      }
    ]
  }
}