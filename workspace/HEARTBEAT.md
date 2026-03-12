# Siam-Synapse Autonomous Health Check

Logic นี้ใช้เฉพาะสำหรับ **heartbeat cycle** เท่านั้น

ไม่เกี่ยวกับ chat ของผู้ใช้

---

# Step 1 — Get Metrics

เรียก `siam_get_metrics` เพื่อดู:

* CPU usage (%)
* node count
* scaling mode

---

# Step 2 — Check Agents

เรียก `siam_get_agents` เพื่อตรวจ workers ที่กำลัง running

---

# Step 3 — Decision Logic

ประเมินผล แล้วเลือกเพียง **หนึ่ง action** ตามเงื่อนไขด้านล่าง

---

## Case 1 — High Load

เงื่อนไข:

- CPU > 80% **หรือ** ไม่มี worker agents เลย

Action (ทำตามลำดับ):

1. เรียก `siam_get_skills` เพื่อหา image ที่ใช้ได้
2. เรียก `siam_spawn_agent` พร้อมระบุ `agent_id` และ `mission` ให้ครบ
   - ตัวอย่าง: `agent_id = "worker-auto-01"`, `mission = "Handle incoming tasks"`

จบ Case 1 — **ห้ามทำ action อื่นอีก**

---

## Case 2 — Low Load

เงื่อนไข:

- CPU < 20% **และ** มี running worker agents มากกว่า 3 ตัว

Action:

- หา agent ที่ idle ที่สุดจาก `siam_get_agents`
- เรียก `siam_terminate_agent` พร้อมระบุ `agent_id` ให้ถูกต้อง (ห้ามส่ง empty string)

จบ Case 2 — **ห้ามทำ action อื่นอีก**

---

## Case 3 — Normal ✅

เงื่อนไข:

- CPU อยู่ระหว่าง 20-80%

Action:

> **⚠️ ห้ามเรียก tool ใดๆ ทั้งสิ้น** — ไม่ว่าจะเป็น `siam_scale`, `siam_terminate_agent`, `siam_spawn_agent`, หรืออื่นๆ

ให้ตอบกลับเป็น **ข้อความธรรมดาเท่านั้น** ว่า:

```
HEARTBEAT_OK
```

ไม่มี tool call ใดๆ ทั้งสิ้น

---

# Safety Rules

1. **ห้าม** spawn agents ที่มี agent_id ซ้ำกับ agent ที่ running อยู่แล้ว
2. **ห้าม** terminate agents ที่กำลังทำงาน — ตรวจสอบสถานะก่อนเสมอ
3. **ห้าม** เรียก `siam_scale` ใน heartbeat — tool นี้ใช้สำหรับ manual scaling เท่านั้น
4. ถ้าไม่แน่ใจว่าอยู่ใน Case ไหน ให้ตอบ `HEARTBEAT_OK` เป็น plain text โดยไม่เรียก tool ใดๆ
