# Siam-Synapse Autonomous Health Check

Logic นี้ใช้เฉพาะสำหรับ **heartbeat cycle**

ไม่เกี่ยวกับ chat ของผู้ใช้

---

# Step 1 — Get Metrics

เรียก

siam_get_metrics

เพื่อดู:

* CPU
* node count
* scaling mode

---

# Step 2 — Check Agents

เรียก

siam_get_agents

เพื่อตรวจ workers

---

# Step 3 — Decision Logic

เลือกเพียง **หนึ่ง action**

---

## Case 1 — High Load

เงื่อนไข:

CPU > 80%

หรือ

ไม่มี worker agents

Action:

1. เรียก siam_get_skills
2. เรียก siam_spawn_agent

รายงานว่า spawn worker ใหม่

---

## Case 2 — Low Load

เงื่อนไข:

CPU < 20%
และ agents > 3

Action:

terminate worker บางตัว

ใช้

siam_terminate_agent

---

## Case 3 — Normal

CPU ระหว่าง 20-80%

Action:

ตอบ

HEARTBEAT_OK

เท่านั้น

---

# Safety Rules

1. ห้าม spawn agents ซ้ำ
2. ห้าม terminate agents ที่กำลังทำงานสำคัญ
3. ถ้าไม่มี action ให้ตอบ

HEARTBEAT_OK

เท่านั้น

---

Add your heartbeat tasks below this line:
- [ ] HEARTBEAT_OK
