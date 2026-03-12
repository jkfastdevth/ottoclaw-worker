# Siam-Synapse Tool Reference

OttoClaw สามารถใช้เครื่องมือต่อไปนี้เพื่อจัดการ Siam-Synapse network

---

# System Tools

## siam_get_metrics

ดูสถานะระบบ

เช่น:

* CPU usage
* node count
* scaling mode

Input:

{}

---

## siam_get_agents

ดู agents ที่กำลังทำงานอยู่

Input:

{}

---

## siam_get_skills

ดู Docker images / skills ที่สามารถใช้ spawn agents ได้

ต้องเรียกก่อน `siam_spawn_agent`

Input:

{}

---

## siam_scale

Scale worker nodes

Input:

{
"action": "up" | "down"
}

---

# Agent Management

## siam_spawn_agent

สร้าง worker agent ใหม่

Input:

{
"agent_id": "unique-name",
"mission": "task description",
"node_ip": "target-node",
"agent_image": "image-from-skills"
}

ใช้เมื่อ:

* ต้องการ worker ระยะยาว
* ต้องการ agent ทำงานต่อเนื่อง

---

## siam_terminate_agent

หยุด agent

Input:

{
"agent_id": "agent-name"
}

---

## siam_find_agents

ค้นหาเพื่อนร่วมทีมที่มีทักษะ (Tools) ที่คุณต้องการ
ใช้สิ่งนี้เมื่อคุณต้องการความช่วยเหลือในเรื่องที่คุณทำไม่ได้ (เช่น การเขียนโค้ด การเข้าเว็บบราวเซอร์)

Input:
{
  "skill": "exec" 
}
(ระบุชื่อ tool ที่ต้องการ เช่น 'exec', 'web', 'siam_send_email')

---

## siam_delegate_mission

มอบหมายงาน (Mission) ระยะยาวให้คนอื่น
ใช้สิ่งนี้แทน siam_send_message เมื่อเป็นงานที่ซับซ้อนและต้องการให้ระบบติดตามสถานะ

Input:
{
  "agent_id": "kaidos",
  "description": "ช่วยเขียนโค้ดส่วนของ backend ให้หน่อย",
  "parent_id": "mission-123" (ใส่ ID ภารกิจปัจจุบันของคุณถ้ามี)
}

---

# OttoClaw One-Shot Jobs

## siam_get_jobs

ดู jobs ทั้งหมด

Input:

{}

---

## siam_submit_job

รันงานแบบ one-shot

เหมาะสำหรับ:

* สรุปข้อมูล
* งาน AI
* งาน compute

Input:

{
"message": "task description",
"model_id": "model name"
}

---

# Node Commands

## siam_run_command

ใช้รันคำสั่งบน node

⚠️ ใช้เฉพาะเมื่อจำเป็นจริง

Input:

{
"node_id": "node name",
"command": "safe command"
}

ห้ามใช้คำสั่งที่อันตราย เช่น

* rm -rf
* shutdown
* format disk
* delete system files

---

## siam_send_message

ส่งข้อความ/คำสั่งหา worker agent ตัวอื่น (ผ่าน Soul name)

ช่วยให้เกิดการร่วมมือกันระหว่าง agent (multi-agent orchestration)

Input:

{
"agent_id": "target-agent-soul-name",
"message": "instruction or command",
"from": "your-name-optional"
}

---

# Global Intelligence (Akashic Library)

## siam_search_memory

ค้นหาข้อมูลใน **Akashic Library (Global Intelligence)** 📚
ใช้เพื่อหาความรู้, ข้อเท็จจริง, หรือบันทึกที่ Agent ตัวอื่นเคยเก็บไว้

Input:

{
"query": "search keywords",
"limit": 5
}

---

## siam_store_memory

บันทึกข้อมูลลงใน **Akashic Library** 🧠
เพื่อให้ความรู้นี้คงอยู่ตลอดไป และ Agent ตัวอื่นสามารถค้นหาได้ในอนาคต

Input:

{
"fact": "information to store",
"confidence": 1.0
}

---

# Tool Execution Rules

1. หากต้องใช้ tool ให้ส่ง tool request
2. รอผลลัพธ์จากระบบก่อนตอบ
3. ห้ามสร้างผลลัพธ์เอง
4. ถ้า tool ไม่จำเป็น อย่าเรียก

---

# Example Workflows

User: "สถานะระบบเป็นยังไง"

→ siam_get_metrics
→ ตอบ CPU และ node count

---

User: "สร้าง worker ใหม่"

→ siam_get_skills
→ siam_spawn_agent

---

User: "รันงานสรุปข่าว"

→ siam_submit_job
