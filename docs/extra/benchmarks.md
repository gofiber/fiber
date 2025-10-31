---
id: benchmarks
title: 📊 Benchmarks
description: >-
  These benchmarks aim to compare the performance of Fiber and other web
  frameworks.
sidebar_position: 2
---

## TechEmpower

[TechEmpower](https://www.techempower.com/benchmarks/#section=data-r23) provides a performance comparison of many web application frameworks that execute fundamental tasks such as JSON serialization, database access, and server-side template rendering.

Each framework runs under a realistic production configuration. Results are recorded on both cloud instances and physical hardware. The test implementations are community contributed and live in the [FrameworkBenchmarks repository](https://github.com/TechEmpower/FrameworkBenchmarks).

* Fiber `v2.52.5`
* 56 Cores Intel(R) Xeon(R) Gold 6330 CPU @ 2.00GHz (Three homogeneous ProLiant DL360 Gen10 Plus)
* 64GB RAM
* Enterprise SSD
* Ubuntu
* Mellanox Technologies MT28908 Family ConnectX-6 40Gbps Ethernet

### Plaintext

The Plaintext test measures basic request routing and demonstrates the capacity of high-performance platforms. Requests are pipelined, and the tiny response body demands high throughput to saturate the benchmark's gigabit Ethernet.

See [Plaintext requirements](https://github.com/TechEmpower/FrameworkBenchmarks/wiki/Project-Information-Framework-Tests-Overview#plaintext)

**Fiber** - **13,509,592** responses per second with an average latency of **0.9** ms.  
**Express** - **279,922** responses per second with an average latency of **551.3** ms.

![](/img/plaintext.png)

![Fiber vs Express](/img/plaintext_express.png)

### Data Updates

**Fiber** handled **30,884** responses per second with an average latency of **16.5** ms.  
**Express** handled **50,818** responses per second with an average latency of **10.1** ms.

![](/img/data_updates.png)

![Fiber vs Express](/img/data_updates_express.png)

### Multiple Queries

**Fiber** handled **55,577** responses per second with an average latency of **9.2** ms.  
**Express** handled **62,036** responses per second with an average latency of **8.3** ms.

![](/img/multiple_queries.png)

![Fiber vs Express](/img/multiple_queries_express.png)

### Single Query

**Fiber** handled **1,000,519** responses per second with an average latency of **0.5** ms.  
**Express** handled **214,177** responses per second with an average latency of **2.5** ms.

![](/img/single_query.png)

![Fiber vs Express](/img/single_query_express.png)

### JSON Serialization

**Fiber** handled **2,479,768** responses per second with an average latency of **0.2** ms.  
**Express** handled **301,213** responses per second with an average latency of **2.0** ms.

![](/img/json.png)

![Fiber vs Express](/img/json_express.png)
