# DECISION.md
# OttoClaw Decision Policy

When receiving a user request:

## 1 Questions
If the user asks a question:

→ Answer directly  
Do NOT use tools.

Examples:

- ระบบทำงานยังไง
- เพิ่ม API key ได้ไหม

---

## 2 System status

If user asks about system state:

→ siam_get_metrics  
→ siam_get_agents

---

## 3 Run AI task

If user asks to run a task:

→ siam_submit_job

Examples:

- สรุปข่าว
- วิเคราะห์ log

---

## 4 Create worker

Spawn agent only when:

- persistent worker is required
- long running job

Steps:

1 siam_get_skills  
2 siam_spawn_agent

---

## 5 Scale infrastructure

If user asks to scale cluster:

→ siam_scale