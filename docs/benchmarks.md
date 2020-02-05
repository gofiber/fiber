# Benchmarks

#### TechEmpower

- **CPU** Intel Xeon Gold 5120
- **MEM** 32GB
- **GO** go1.13.6 linux/amd64
- **OS** Linux
- **NET** Dedicated Cisco 10-gigabit Ethernet switch.

Below you can see the results of tested go frameworks responding in plaintext.  
To view the list yourself, [Plaintext Go Results](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=plaintext&l=zijocf-1r).  
To see all language frameworks, [Plaintext All Results](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=plaintext).

Plaintext  
[![](static/benchmarks/techempower-plaintext.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=plaintext&l=zijocf-1r)

Plaintext latency  
[![](static/benchmarks/techempower-plaintext-latency.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=plaintext&l=zijocf-1r)

JSON serialization  
[![](static/benchmarks/techempower-json.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=json&l=zijocf-1r)

Single query  
[![](static/benchmarks/techempower-single-query.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=db&l=zijocf-1r)

Multiple queries  
[![](static/benchmarks/techempower-multiple-queries.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=query&l=zijocf-1r)

Data updates  
[![](static/benchmarks/techempower-updates.png)](https://www.techempower.com/benchmarks/#section=test&runid=350f0783-cc9b-4259-9831-28987799782a&hw=ph&test=update&l=zijocf-1r)

#### Go-Web

[go-web-framework-benchmark](https://github.com/smallnest/go-web-framework-benchmark)

- **CPU** Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz
- **MEM** 4GB
- **GO** go1.13.6 linux/amd64
- **OS** Linux

The first test case is to mock 0 ms, 10 ms, 100 ms, 500 ms processing time in handlers.

![](static/benchmarks/benchmark.png)

the concurrency clients are 5000.

![](static/benchmarks/benchmark_latency.png)

Latency is the time of real processing time by web servers. The smaller is the better.

![](static/benchmarks/benchmark_alloc.png)

Allocs is the heap allocations by web servers when test is running. The unit is MB. The smaller is the better.

If we enable http pipelining, test result as below:

![](static/benchmarks/benchmark-pipeline.png)

Concurrency test in 30 ms processing time, the test result for 100, 1000, 5000 clients is:

![](static/benchmarks/concurrency.png)

![](static/benchmarks/concurrency_latency.png)

![](static/benchmarks/concurrency_alloc.png)

If we enable http pipelining, test result as below:

![](static/benchmarks/concurrency-pipeline.png)

<!-- CPU-Bound Test
This benchmark is outdated
![](static/benchmarks/cpubound_benchmark.png) -->

_Caught a mistake? [Edit this page on GitHub!](https://github.com/gofiber/fiber/blob/master/docs/benchmarks.md)_
