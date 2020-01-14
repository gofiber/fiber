# Benchmarks
Benchmarks are performed by [go-web-framework-benchmark](https://github.com/smallnest/go-web-framework-benchmark) on a digitalocean machine.

* **CPU** Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz
* **MEM** 4GB
* **GO** go1.13.6 linux/amd64
* **OS** Ubuntu 18.04.3 LTS  

#### Basic Test
The first test case is to mock 0 ms, 10 ms, 100 ms, 500 ms processing time in handlers.  

![](static/benchmarks/benchmark.png)

the concurrency clients are 5000.

![](static/benchmarks/benchmark_latency.png)

Latency is the time of real processing time by web servers. The smaller is the better.

![](static/benchmarks/benchmark_alloc.png)

Allocs is the heap allocations by web servers when test is running. The unit is MB. The smaller is the better.

If we enable http pipelining, test result as below:

![](static/benchmarks/benchmark-pipeline.png)

#### Concurrency Test
In 30 ms processing time, the test result for 100, 1000, 5000 clients is:

![](static/benchmarks/concurrency.png)

![](static/benchmarks/concurrency_latency.png)

![](static/benchmarks/concurrency_alloc.png)

If we enable http pipelining, test result as below:

![](static/benchmarks/concurrency-pipeline.png)

#### CPU-Bound Test

![](static/benchmarks/cpubound_benchmark.png)
