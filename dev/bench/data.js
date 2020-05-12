window.BENCHMARK_DATA = {
  "lastUpdate": 1589326697287,
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
      }
    ]
  }
}