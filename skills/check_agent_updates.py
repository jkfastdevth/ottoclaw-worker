import os
import json
import urllib.request
import urllib.error

def get_skill_info():
    return {
        "name": "check_agent_updates",
        "description": "Checks all registered agents for available updates and optionally triggers update for outdated agents. Use this to audit which agents need updating or to push updates fleet-wide.",
        "parameters": {
            "type": "object",
            "properties": {
                "trigger_update": {
                    "type": "boolean",
                    "description": "If true, automatically send 'ottoclaw update' to every agent that is outdated. Default: false (report only)."
                },
                "agent_id": {
                    "type": "string",
                    "description": "If specified, only check/update this specific agent instead of all agents."
                }
            }
        }
    }


def _api_request(method, path, api_url, api_key, body=None):
    url = api_url.rstrip("/") + path
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("X-API-Key", api_key)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode()), resp.status
    except urllib.error.HTTPError as e:
        return json.loads(e.read().decode() or "{}"), e.code
    except Exception as e:
        return {"error": str(e)}, 0


def execute(params):
    trigger_update = params.get("trigger_update", False)
    target_agent_id = params.get("agent_id", "")

    api_url = os.getenv("MASTER_API_URL", "http://siam-synapse-master:8080/api/agent/v1")
    api_key = os.getenv("MASTER_API_KEY") or os.getenv("OTTOCLAW_API_KEY", "")

    if not api_key:
        return {"status": "error", "message": "MASTER_API_KEY not set"}

    # 1. Fetch all agents
    data, status = _api_request("GET", "/agents", api_url, api_key)
    if status != 200:
        return {"status": "error", "message": f"Failed to list agents: {data}"}

    agents = data.get("agents", [])
    if not agents:
        return {"status": "success", "message": "No agents found", "agents": []}

    # 2. Filter if specific agent requested
    if target_agent_id:
        agents = [a for a in agents if a.get("id", "").lower() == target_agent_id.lower()]
        if not agents:
            return {"status": "error", "message": f"Agent '{target_agent_id}' not found"}

    # 3. Evaluate update status per agent
    results = []
    outdated_count = 0
    updated_count = 0

    for agent in agents:
        agent_id = agent.get("id", "")
        version = agent.get("version", "") or "unknown"
        latest = agent.get("latest_version", "") or ""
        agent_status = agent.get("status", "")

        needs_update = latest != "" and version != "unknown" and version != latest
        if needs_update:
            outdated_count += 1

        entry = {
            "id": agent_id,
            "status": agent_status,
            "version": version,
            "latest_version": latest if latest else "unknown",
            "needs_update": needs_update,
        }

        # 4. Trigger update if requested and agent is outdated
        if trigger_update and needs_update:
            upd_data, upd_status = _api_request(
                "POST", f"/agent-update-remote/{agent_id}", api_url, api_key
            )
            if upd_status == 200:
                entry["update_triggered"] = True
                updated_count += 1
            else:
                entry["update_triggered"] = False
                entry["update_error"] = upd_data.get("error", "unknown error")

        results.append(entry)

    summary = f"{len(agents)} agents checked, {outdated_count} outdated"
    if trigger_update:
        summary += f", {updated_count} update(s) triggered"

    return {
        "status": "success",
        "summary": summary,
        "agents": results,
        "outdated_count": outdated_count,
    }
