---
id: benchmarks
title: 📊 Benchmarks
description: >-
  These benchmarks aim to compare the performance of Fiber and other web
  frameworks.
sidebar_position: 2
---

## TechEmpower

[TechEmpower](https://www.techempower.com/benchmarks/#section=data-r19&hw=ph&test=composite) provides a performance comparison of many web application frameworks executing fundamental tasks such as JSON serialization, database access, and server-side template composition.

Each framework is operating in a realistic production configuration. Results are captured on cloud instances and on physical hardware. The test implementations are largely community-contributed and all source is available at the [GitHub repository](https://github.com/TechEmpower/FrameworkBenchmarks).

* Fiber `v1.10.0`
* 28 HT Cores Intel\(R\) Xeon\(R\) Gold 5120 CPU @ 2.20GHz
* 32GB RAM
* Ubuntu 18.04.3 4.15.0-88-generic
* Dedicated Cisco 10-Gbit Ethernet switch.

### Plaintext

The Plaintext test is an exercise of the request-routing fundamentals only, designed to demonstrate the capacity of high-performance platforms in particular. Requests will be sent using HTTP pipelining. The response payload is still small, meaning good performance is still necessary in order to saturate the gigabit Ethernet of the test environment.

See [Plaintext requirements](https://github.com/TechEmpower/FrameworkBenchmarks/wiki/Project-Information-Framework-Tests-Overview#single-database-query)

**Fiber** - **6,162,556** responses per second with an average latency of **2.0** ms.  
**Express** - **367,069** responses per second with an average latency of **354.1** ms.

![](/img/plaintext.png)

![Fiber vs Express](/img/plaintext_express.png)

### Data Updates

**Fiber** handled **11,846** responses per second with an average latency of **42.8** ms.  
**Express** handled **2,066** responses per second with an average latency of **390.44** ms.

![](/img/data_updates.png)

![Fiber vs Express](/img/data_updates_express.png)

### Multiple Queries

**Fiber** handled **19,664** responses per second with an average latency of **25.7** ms.  
**Express** handled **4,302** responses per second with an average latency of **117.2** ms.

![](/img/multiple_queries.png)

![Fiber vs Express](/img/multiple_queries_express.png)

### Single Query

**Fiber** handled **368,647** responses per second with an average latency of **0.7** ms.  
**Express** handled **57,880** responses per second with an average latency of **4.4** ms.

![](/img/single_query.png)

![Fiber vs Express](/img/single_query_express.png)

### JSON Serialization

**Fiber** handled **1,146,667** responses per second with an average latency of **0.4** ms.  
**Express** handled **244,847** responses per second with an average latency of **1.1** ms.

![](/img/json.png)

![Fiber vs Express](/img/json_express.png)

## Go web framework benchmark

🔗 [https://github.com/smallnest/go-web-framework-benchmark](https://github.com/smallnest/go-web-framework-benchmark)

* **CPU** Intel\(R\) Xeon\(R\) Gold 6140 CPU @ 2.30GHz
* **MEM** 4GB
* **GO** go1.13.6 linux/amd64
* **OS** Linux

The first test case is to mock **0 ms**, **10 ms**, **100 ms**, **500 ms** processing time in handlers.

![](/img/benchmark.png)

The concurrency clients are **5000**.

![](/img/benchmark_latency.png)

Latency is the time of real processing time by web servers. _The smaller is the better._

![](/img/benchmark_alloc.png)

Allocs is the heap allocations by web servers when test is running. The unit is MB. _The smaller is the better._

If we enable **http pipelining**, test result as below:

![](/img/benchmark-pipeline.png)

Concurrency test in **30 ms** processing time, the test result for **100**, **1000**, **5000** clients is:

![](/img/concurrency.png)

![](/img/concurrency_latency.png)

![](/img/concurrency_alloc.png)

If we enable **http pipelining**, test result as below:

![](/img/concurrency-pipeline.png)

Dependency graph for `v1.9.0`

![](/img/graph.svg)
