A collection of common functions but with better performance, less allocations and no dependencies created for [Fiber](https://github.com/gofiber/fiber).

```go
// go test -benchmem -run=^$ -bench=Benchmark_ -count=2
goos: windows
goarch: amd64
pkg: github.com/gofiber/fiber/v3/utils
cpu: AMD Ryzen 7 5800X 8-Core Processor
Benchmark_ToLowerBytes/fiber-16                 51138252                22.61 ns/op            0 B/op          0 allocs/op
Benchmark_ToLowerBytes/fiber-16                 52126545                22.63 ns/op            0 B/op          0 allocs/op
Benchmark_ToLowerBytes/default-16               16114736                72.76 ns/op           80 B/op          1 allocs/op
Benchmark_ToLowerBytes/default-16               16651540                73.85 ns/op           80 B/op          1 allocs/op
Benchmark_ToUpperBytes/fiber-16                 52127224                22.62 ns/op            0 B/op          0 allocs/op
Benchmark_ToUpperBytes/fiber-16                 54283167                22.86 ns/op            0 B/op          0 allocs/op
Benchmark_ToUpperBytes/default-16               14060098                84.12 ns/op           80 B/op          1 allocs/op
Benchmark_ToUpperBytes/default-16               14183122                84.51 ns/op           80 B/op          1 allocs/op
Benchmark_EqualFoldBytes/fiber-16               29240264                41.22 ns/op            0 B/op          0 allocs/op
Benchmark_EqualFoldBytes/fiber-16               28535826                40.84 ns/op            0 B/op          0 allocs/op
Benchmark_EqualFoldBytes/default-16              7929867               150.2 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFoldBytes/default-16              7935478               149.7 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFold/fiber-16                    35442768                34.25 ns/op            0 B/op          0 allocs/op
Benchmark_EqualFold/fiber-16                    35946870                34.96 ns/op            0 B/op          0 allocs/op
Benchmark_EqualFold/default-16                   8942130               133.5 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFold/default-16                   8977231               134.3 ns/op             0 B/op          0 allocs/op
Benchmark_UUID/fiber-16                         30726213                40.57 ns/op           48 B/op          1 allocs/op
Benchmark_UUID/fiber-16                         26539394                40.25 ns/op           48 B/op          1 allocs/op
Benchmark_UUID/default-16                        4737199               247.5 ns/op           168 B/op          6 allocs/op
Benchmark_UUID/default-16                        4603738               250.8 ns/op           168 B/op          6 allocs/op
Benchmark_ConvertToBytes/fiber-16               62450884                19.41 ns/op            0 B/op          0 allocs/op
Benchmark_ConvertToBytes/fiber-16               52123602                19.53 ns/op            0 B/op          0 allocs/op
Benchmark_UnsafeString/unsafe-16                1000000000               0.4496 ns/op          0 B/op          0 allocs/op
Benchmark_UnsafeString/unsafe-16                1000000000               0.4488 ns/op          0 B/op          0 allocs/op
Benchmark_UnsafeString/default-16               79925935                13.99 ns/op           16 B/op          1 allocs/op
Benchmark_UnsafeString/default-16               85637211                14.35 ns/op           16 B/op          1 allocs/op
Benchmark_UnsafeBytes/unsafe-16                 540970148                2.214 ns/op           0 B/op          0 allocs/op
Benchmark_UnsafeBytes/unsafe-16                 543356940                2.212 ns/op           0 B/op          0 allocs/op
Benchmark_UnsafeBytes/default-16                68896224                17.19 ns/op           16 B/op          1 allocs/op
Benchmark_UnsafeBytes/default-16                70560426                17.05 ns/op           16 B/op          1 allocs/op
Benchmark_ToString-16                           29504036                39.57 ns/op           40 B/op          2 allocs/op
Benchmark_ToString-16                           30738334                38.89 ns/op           40 B/op          2 allocs/op
Benchmark_GetMIME/fiber-16                      28207086                41.84 ns/op            0 B/op          0 allocs/op
Benchmark_GetMIME/fiber-16                      28165773                41.83 ns/op            0 B/op          0 allocs/op
Benchmark_GetMIME/default-16                    12583132                94.04 ns/op            0 B/op          0 allocs/op
Benchmark_GetMIME/default-16                    12829614                93.50 ns/op            0 B/op          0 allocs/op
Benchmark_ParseVendorSpecificContentType/vendorContentType-16           30267411                38.72 ns/op           16 B/op          1 allocs/op
Benchmark_ParseVendorSpecificContentType/vendorContentType-16           28543563                38.60 ns/op           16 B/op          1 allocs/op
Benchmark_ParseVendorSpecificContentType/defaultContentType-16          249869286                4.830 ns/op           0 B/op          0 allocs/op
Benchmark_ParseVendorSpecificContentType/defaultContentType-16          248999592                4.805 ns/op           0 B/op          0 allocs/op
Benchmark_StatusMessage/fiber-16                                        1000000000               0.6744 ns/op          0 B/op          0 allocs/op
Benchmark_StatusMessage/fiber-16                                        1000000000               0.6788 ns/op          0 B/op          0 allocs/op
Benchmark_StatusMessage/default-16                                      446818872                2.664 ns/op           0 B/op          0 allocs/op
Benchmark_StatusMessage/default-16                                      447009616                2.661 ns/op           0 B/op          0 allocs/op
Benchmark_ToUpper/fiber-16                                              20480331                56.50 ns/op           80 B/op          1 allocs/op
Benchmark_ToUpper/fiber-16                                              21541200                56.65 ns/op           80 B/op          1 allocs/op
Benchmark_ToUpper/default-16                                             8433409               141.2 ns/op            80 B/op          1 allocs/op
Benchmark_ToUpper/default-16                                             8473737               141.1 ns/op            80 B/op          1 allocs/op
Benchmark_ToLower/fiber-16                                              27248326                44.68 ns/op           80 B/op          1 allocs/op
Benchmark_ToLower/fiber-16                                              26918443                44.70 ns/op           80 B/op          1 allocs/op
Benchmark_ToLower/default-16                                             8447336               141.9 ns/op            80 B/op          1 allocs/op
Benchmark_ToLower/default-16                                             8423156               140.6 ns/op            80 B/op          1 allocs/op
```
