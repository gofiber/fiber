---
id: route-matcher
title: 🎯 Route Matcher
description: >-
  Interactive playground for Fiber's route matching: edit a route table and a
  request, see which route wins, why the others lose, and which parameters
  are extracted.
sidebar_position: 2.5
---

import RoutePlayground from '@site/src/components/route-playground';

Edit the route table or the request and watch which route wins, why the others lose, and which parameters are extracted. Routes are tried in registration order, exactly like in a real Fiber app:

<RoutePlayground />

:::note
The playground simulates Fiber's matcher with default settings (case-insensitive, non-strict routing). `datetime` and custom constraints are not simulated, and `regex()` runs on the JS engine instead of Go's RE2.
:::

The [routing guide](../guide/routing.md) explains the full syntax: [parameters](../guide/routing.md#parameters), wildcards, literal separators, and [constraints](../guide/routing.md#constraints).
