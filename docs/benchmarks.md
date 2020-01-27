# Benchmarks

#### TechEmpower
* **CPU**  Intel Xeon Gold 5120  
* **MEM** 32GB  
* **GO** go1.13.6 linux/amd64  
* **OS** Linux  
* **NET** Dedicated Cisco 10-gigabit Ethernet switch.


Below you can see the results of tested go frameworks responding in plaintext.  
To view the list yourself, [Plaintext Go Results](https://www.techempower.com/benchmarks/#section=test&runid=8721f3a4-7b13-4703-9cd8-91b6779668c2&hw=ph&test=plaintext&l=zijocf-1r).  
To see all language frameworks, [Plaintext All Results](https://www.techempower.com/benchmarks/#section=test&runid=8721f3a4-7b13-4703-9cd8-91b6779668c2&hw=ph&test=plaintext).  

![](static/benchmarks/techempower-plaintext.png)

![](static/benchmarks/techempower-plaintext-latency.png)

![](static/benchmarks/techempower-json.png)

![](static/benchmarks/techempower-single-query.png)

![](static/benchmarks/techempower-multiple-queries.png)

![](static/benchmarks/techempower-updates.png)

#### Go-Web
[go-web-framework-benchmark](https://github.com/smallnest/go-web-framework-benchmark)

* **CPU** Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz
* **MEM** 4GB
* **GO** go1.13.6 linux/amd64
* **OS** Linux  

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

CPU-Bound Test  

![](static/benchmarks/cpubound_benchmark.png)

*Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/benchmarks.md)*
