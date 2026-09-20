---
id: crud-and-health
title: 🚀 Minimal CRUD and Health Check
description: >-
  A minimal in-memory CRUD example with a /health endpoint. Useful as a
  starting point for learning, smoke tests, and CI checks. Not for production
  persistence.
sidebar_position: 12
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
This guide shows a minimal Fiber app with two things most "hello world" examples leave out:

- A `/health` endpoint that returns `{ "status": "ok" }`
- A tiny in-memory CRUD for `Item` (`list`, `create`, `get`, `update`, `delete`)

:::caution
This example stores data in memory. It is intended for **learning, smoke tests, and CI checks**, not for production persistence. Restarting the app clears all data.
:::

## Full example

<Tabs>
<TabItem value="example" label="Example">
