# Siam-Synapse Orchestrator Tools

คุณมีเครื่องมือต่อไปนี้สำหรับจัดการ Siam-Synapse network:

## System Tools

### `siam_get_metrics`
ดู CPU, node count, scaling mode ปัจจุบัน
```
{}
```

### `siam_get_agents`
ดู sub-agents ทั้งหมดที่กำลังทำงาน
```
{}
```

### `siam_get_skills`
ดู skills และ Docker images ที่ใช้ได้ — **เรียกก่อน spawn เสมอ**
```
{}
```

### `siam_scale`
Scale workers ขึ้น/ลง
```json
{"action": "up" | "down"}
```

## Agent Management

### `siam_spawn_agent`
Spawn Docker worker agent ใหม่
```json
{
  "agent_id": "unique-name",
  "mission": "focused task description",
  "node_ip": "local",
  "agent_image": "docker-image (from siam_get_skills)"
}
```

### `siam_terminate_agent`
หยุดและลบ agent
```json
{"agent_id": "name-to-stop"}
```

## OttoClaw One-Shot Jobs

### `siam_get_jobs`
ดู one-shot jobs ทั้งหมด (state: deployed/running/succeeded/failed)
```
{}
```

### `siam_submit_job`
Submit one-shot job: สร้าง container ใหม่, ส่ง LLM message, จับ output, จบแล้ว exit
(ใช้สำหรับการสั่งงานแบบ Auto-dispatch: โยนภารกิจให้ Master จัดหา Worker/Skill มารันให้แบบ "Fire and Forget")
```json
{
  "message": "task to run",
  "model_id": "llama-3.3-70b-versatile"
}
```

### `siam_run_command`
ส่งคำสั่ง Shell ไปรันที่ Node ปลายทางโดยตรง (เช่น 'ls -la', 'ps aux')
```json
{
  "node_id": "target-node-name",
  "command": "shell command to run"
}
```

## ตัวอย่าง Workflow

### User: "สถานะระบบเป็นยังไง"
1. เรียก `siam_get_metrics` → รายงาน CPU, nodes
2. เรียก `siam_get_agents` → รายงาน agents ที่ทำงาน

### User: "สร้าง agent ใหม่ทำ X"
1. `siam_get_skills` — หา image ที่เหมาะสม
2. `siam_spawn_agent` — spawn ด้วย mission ที่ชัดเจน
3. แจ้ง job ID กลับผู้ใช้

### User: "รัน task แบบ one-shot"
1. `siam_submit_job` — ส่ง message
2. แจ้ง job_id กลับ, ผู้ใช้ตามดูสถานะได้ที่ dashboard หรือ `siam_get_jobs`
