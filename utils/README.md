A collection of common functions but with better performance, less allocations and no dependencies created for [Fiber](https://github.com/gofiber/fiber).

```go
// go test -benchmem -run=^$ -bench=Benchmark_ -count=2

Benchmark_ToLowerBytes/fiber-16                 42847654                25.7 ns/op             0 B/op          0 allocs/op
Benchmark_ToLowerBytes/fiber-16                 46143196                25.7 ns/op             0 B/op          0 allocs/op
Benchmark_ToLowerBytes/default-16               17387322                67.4 ns/op            48 B/op          1 allocs/op
Benchmark_ToLowerBytes/default-16               17906491                67.4 ns/op            48 B/op          1 allocs/op

Benchmark_ToUpperBytes/fiber-16                 46143729                25.7 ns/op             0 B/op          0 allocs/op
Benchmark_ToUpperBytes/fiber-16                 47989250                25.6 ns/op             0 B/op          0 allocs/op
Benchmark_ToUpperBytes/default-16               15580854                76.7 ns/op            48 B/op          1 allocs/op
Benchmark_ToUpperBytes/default-16               15381202                76.9 ns/op            48 B/op          1 allocs/op

Benchmark_TrimRightBytes/fiber-16               70572459                16.3 ns/op             8 B/op          1 allocs/op
Benchmark_TrimRightBytes/fiber-16               74983597                16.3 ns/op             8 B/op          1 allocs/op
Benchmark_TrimRightBytes/default-16             16212578                74.1 ns/op            40 B/op          2 allocs/op
Benchmark_TrimRightBytes/default-16             16434686                74.1 ns/op            40 B/op          2 allocs/op

Benchmark_TrimLeftBytes/fiber-16                74983128                16.3 ns/op             8 B/op          1 allocs/op
Benchmark_TrimLeftBytes/fiber-16                74985002                16.3 ns/op             8 B/op          1 allocs/op
Benchmark_TrimLeftBytes/default-16              21047868                56.5 ns/op            40 B/op          2 allocs/op
Benchmark_TrimLeftBytes/default-16              21048015                56.5 ns/op            40 B/op          2 allocs/op

Benchmark_TrimBytes/fiber-16                    54533307                21.9 ns/op            16 B/op          1 allocs/op
Benchmark_TrimBytes/fiber-16                    54532812                21.9 ns/op            16 B/op          1 allocs/op
Benchmark_TrimBytes/default-16                  14282517                84.6 ns/op            48 B/op          2 allocs/op
Benchmark_TrimBytes/default-16                  14114508                84.7 ns/op            48 B/op          2 allocs/op

Benchmark_EqualFolds/fiber-16                   36355153                32.6 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFolds/fiber-16                   36355593                32.6 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFolds/default-16                 15186220                78.1 ns/op             0 B/op          0 allocs/op
Benchmark_EqualFolds/default-16                 15186412                78.3 ns/op             0 B/op          0 allocs/op

Benchmark_UUID/fiber-16                         23994625                49.8 ns/op            48 B/op          1 allocs/op
Benchmark_UUID/fiber-16                         23994768                50.1 ns/op            48 B/op          1 allocs/op
Benchmark_UUID/default-16                        3233772                 371 ns/op           208 B/op          6 allocs/op
Benchmark_UUID/default-16                        3251295                 370 ns/op           208 B/op          6 allocs/op

Benchmark_GetString/unsafe-16                 1000000000               0.709 ns/op             0 B/op          0 allocs/op
Benchmark_GetString/unsafe-16                 1000000000               0.713 ns/op             0 B/op          0 allocs/op
Benchmark_GetString/default-16                  59986202                19.0 ns/op            16 B/op          1 allocs/op
Benchmark_GetString/default-16                  63142939                19.0 ns/op            16 B/op          1 allocs/op

Benchmark_GetBytes/unsafe-16                   508360195                2.36 ns/op             0 B/op          0 allocs/op
Benchmark_GetBytes/unsafe-16                   508359979                2.35 ns/op             0 B/op          0 allocs/op
Benchmark_GetBytes/default-16                   46143019                25.7 ns/op            16 B/op          1 allocs/op
Benchmark_GetBytes/default-16                   44434734                25.6 ns/op            16 B/op          1 allocs/op

Benchmark_GetMIME/fiber-16                      21423750                56.3 ns/op             0 B/op          0 allocs/op
Benchmark_GetMIME/fiber-16                      21423559                55.4 ns/op             0 B/op          0 allocs/op
Benchmark_GetMIME/default-16                     6735282                 173 ns/op             0 B/op          0 allocs/op
Benchmark_GetMIME/default-16                     6895002                 172 ns/op             0 B/op          0 allocs/op

Benchmark_StatusMessage/fiber-16              1000000000               0.766 ns/op             0 B/op          0 allocs/op
Benchmark_StatusMessage/fiber-16              1000000000               0.767 ns/op             0 B/op          0 allocs/op
Benchmark_StatusMessage/default-16             159538528                7.50 ns/op             0 B/op          0 allocs/op
Benchmark_StatusMessage/default-16             159750830                7.51 ns/op             0 B/op          0 allocs/op

Benchmark_ToUpper/fiber-16                      22217408                53.3 ns/op            48 B/op          1 allocs/op
Benchmark_ToUpper/fiber-16                      22636554                53.2 ns/op            48 B/op          1 allocs/op
Benchmark_ToUpper/default-16                    11108600                 108 ns/op            48 B/op          1 allocs/op
Benchmark_ToUpper/default-16                    11108580                 108 ns/op            48 B/op          1 allocs/op

Benchmark_ToLower/fiber-16                      23994720                49.8 ns/op            48 B/op          1 allocs/op
Benchmark_ToLower/fiber-16                      23994768                50.1 ns/op            48 B/op          1 allocs/op
Benchmark_ToLower/default-16                    10808376                 110 ns/op            48 B/op          1 allocs/op
Benchmark_ToLower/default-16                    10617034                 110 ns/op            48 B/op          1 allocs/op

Benchmark_TrimRight/fiber-16                   413699521                2.94 ns/op             0 B/op          0 allocs/op
Benchmark_TrimRight/fiber-16                   415131687                2.91 ns/op             0 B/op          0 allocs/op
Benchmark_TrimRight/default-16                  23994577                49.1 ns/op            32 B/op          1 allocs/op
Benchmark_TrimRight/default-16                  24484249                49.4 ns/op            32 B/op          1 allocs/op

Benchmark_TrimLeft/fiber-16                    379661170                3.13 ns/op             0 B/op          0 allocs/op
Benchmark_TrimLeft/fiber-16                    382079941                3.16 ns/op             0 B/op          0 allocs/op
Benchmark_TrimLeft/default-16                   27900877                41.9 ns/op            32 B/op          1 allocs/op
Benchmark_TrimLeft/default-16                   28564898                42.0 ns/op            32 B/op          1 allocs/op

Benchmark_Trim/fiber-16                        236632856                 4.96 ns/op            0 B/op          0 allocs/op
Benchmark_Trim/fiber-16                        237570085                 4.93 ns/op            0 B/op          0 allocs/op
Benchmark_Trim/default-16                       18457221                 66.0 ns/op           32 B/op          1 allocs/op
Benchmark_Trim/default-16                       18177328                 65.9 ns/op           32 B/op          1 allocs/op
Benchmark_Trim/default.trimspace-16            188933770                 6.33 ns/op            0 B/op          0 allocs/op
Benchmark_Trim/default.trimspace-16            184007649                 6.42 ns/op            0 B/op          0 allocs/op

Benchmark_ConvertToBytes/fiber-8                43773547                24.43 ns/op            0 B/op          0 allocs/op
Benchmark_ConvertToBytes/fiber-8                45849477                25.33 ns/op            0 B/op          0 allocs/op
```
