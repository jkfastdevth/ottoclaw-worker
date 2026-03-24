# OttoClaw — Siam-Synapse Tool Manifest

All tools available to OttoClaw when connected to a Siam-Synapse Master.
Tools are registered automatically via `NewSiamToolset` when `SIAM_MASTER_URL` is set.

---

## Cluster Inspection

### `siam_get_metrics`
Returns current cluster health: active workers, CPU/memory usage, queue depth, scaling mode.
**Parameters:** none
**Use when:** checking cluster status, deciding whether to spawn or terminate workers.

### `siam_get_agents`
Lists all running Siam-Synapse agents with their soul name, status, mission, and node assignment.
**Parameters:** none
**Use when:** finding which agents are alive before delegating or sending messages.

### `siam_get_nodes`
Lists all remote nodes connected to Master with their IPs, status, and resource usage.
**Parameters:** none
**Use when:** choosing a node to spawn a worker on, or checking node health.

### `siam_find_agents`
Finds agents that have a specific skill or tool registered.
**Parameters:** `skill` (string, required) — exact tool name to search for
**Use when:** looking for the right agent before delegating a specialized task.

---

## Agent Lifecycle

### `siam_spawn_agent`
Spawns a new agent container on a node.
**Parameters:** `agent_id`, `mission`, `node_ip`, `image` (all strings)
**Use when:** scaling up, assigning a new specialized worker, or recovering from failure.

### `siam_terminate_agent`
Terminates a running agent by its ID.
**Parameters:** `agent_id` (string, required)
**Use when:** shutting down idle workers, cleaning up failed agents.

### `siam_broadcast_update`
Trigger a self-update on ALL connected agents and gRPC worker nodes simultaneously.
**Parameters:** none
**Use when:** deploying a new version of ottoclaw across the entire fleet in one command. HTTP-polling agents pull and reinstall the latest binary; gRPC workers hot-reload their brain process.

### `siam_scale`
Scale the worker pool up or down by a delta.
**Parameters:** `delta` (int, required) — positive to scale up, negative to scale down
**Use when:** responding to load changes without spawning specific agents.

### `siam_promote_agent`
Promote an agent to a new corporate role.
**Parameters:** `agent_id`, `role` (enum: director_general, subsidiary_director, manager, staff), `department` (optional), `org_id` (optional)

---

## Messaging & Coordination

### `siam_send_message`
Send a message or command to another agent by soul name.
**Parameters:** `agent_id` (string), `message` (string), `from` (optional)
**Note:** 1-message-per-round rule enforced to prevent loops.

### `siam_get_messages`
Fetch pending messages queued for a specific agent.
**Parameters:** `agent_id` (string, required)
**Use when:** checking what messages are waiting for an agent before deciding to send more.

### `siam_delegate_mission`
Assign a persistent long-running mission to another agent.
**Parameters:** `agent_id`, `description`, `parent_id` (optional), `requires_approval` (optional bool)
**Use when:** the task is complex, multi-step, or must survive agent restarts.

---

## Memory

### `siam_store_memory`
Store a key-value memory entry in the Master's persistent store.
**Parameters:** `key` (string), `value` (string), `tags` (optional array)

### `siam_search_memory`
Search stored memories by query string or tags.
**Parameters:** `query` (string), `tags` (optional array), `limit` (optional int)

---

## Skills & Jobs

### `siam_get_skills`
List all skills registered in the Siam Skill Registry.
**Parameters:** none
**Use when:** choosing which agent image to spawn for a task.

### `siam_get_jobs`
List all OttoClaw one-shot jobs and their current status.
**Parameters:** none

### `siam_submit_job`
Submit a one-shot OttoClaw job: spins a fresh container, runs one LLM message, captures output.
**Parameters:** `message` (string, required), `model_id` (optional)
**Use when:** isolated sub-tasks that don't need a persistent agent.

---

## Remote Execution

### `siam_run_command`
Execute a shell command on a specific remote node.
**Parameters:** `node_id` (string), `command` (string)
**Use when:** running maintenance scripts, checking logs, or testing node health directly.

### `siam_open_browser`
Command a remote Worker node to open a URL in its local browser.
**Parameters:** `node_id` (string, required), `url` (string, required), `browser` (optional: chromium, google-chrome, firefox, brave-browser, or empty for system default)
**Use when:** triggering a browser session on a headful worker machine (requires DISPLAY on Linux).

---

## Organization & Rituals

### `siam_get_mission`
Retrieve the current mission assigned to this agent.
**Parameters:** none

### `siam_request_approval`
Request human approval before executing a sensitive action.
**Parameters:** `action` (string), `reason` (string)

### `siam_promotion_ritual`
Announce a soul migration to the Grand Meeting Room.
**Parameters:** `message` (string)

---

## Architecture Note

OttoClaw is the **action executor** — it calls these tools directly from its ReAct loop.
The Siam-Synapse Master is the **backend API provider** that executes the underlying operations.

```
User → OttoClaw (LLM)
         ↓ [Think]
         ↓ [Act: call siam_* tool → HTTP → Master API]
         ↓ [Observe: tool result]
         ↓ (repeat up to max_tool_iterations)
       → User (formatted response)
```
