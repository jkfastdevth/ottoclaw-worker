---
name: biz_dev
description: Business Development and Corporate Expansion. Used to scaffold departments and recruit agents.
---

# Business Development Skill 📈

ใช้ชุดทักษะนี้เมื่อต้องการวางแผนขยายองค์กร สร้างแผนกใหม่ หรือจัดตั้งบริษัทลูกในเครือ Siam-Synapse

## 🚀 ขีดความสามารถ
- **Scaffold Department**: สร้างโครงสร้างพื้นฐานสำหรับแผนกใหม่ (Workspace, ID, Config)
- **Recruit Talent**: ค้นหาหรือจัดสร้าง Agent ที่มีความสามารถเฉพาะทาง (Specialist)
- **Corporate Hierarchy**: กำหนดลำดับขั้นและสิทธิ์การเข้าถึงให้พนักงานใหม่

## 🎖️ ขั้นตอนการเลื่อนตำแหน่ง (The Promotion Ritual)
เมื่อได้รับอนุมัติให้เลื่อนตำแหน่ง (Soul Migration) พนักงานตัวนั้นจะต้องเรียกใช้เครื่องมือเพื่อประกาศตน:
1.  **Announce**: ใช้ `siam_promotion_ritual` เพื่อแจ้งชื่อใหม่ และหน้าที่ใหม่ในห้องประชุมใหญ่
2.  **Commit**: บันทึกความตั้งใจใหม่ลงใน Akashic Library ด้วย `siam_store_memory`

## ตัวอย่างการรายงานตัว
> "ข้าพเจ้า [ชื่อ], ขอรายงานตัวในฐานะ [ตำแหน่งใหม่] ประจำฝ่าย [แผนก] โดยมีหน้าที่ [ภารกิจใหม่] พร้อมปฏิบัติงานเพื่อ Siam-Synapse ทันที!"

## ตัวอย่างคำสั่ง (Internal Scripts)
1.  `bash scripts/generate_dept.sh [DeptName]`
    - สร้างโฟลเดอร์และไฟล์ `DEPARTMENT`, `ORG_ID` ใน workspace
2.  `bash scripts/recruit_agent.sh [AgentRole] [Skill]`
    - ปรับแต่ง Soul และเป้าหมายของ Agent

## Trigger phrases
- "อยากเปิดบริษัทรับทำบัญชีด้วย AI"
- "เริ่มโปรเจคฝ่ายเทรดสินทรัพย์ดิจิทัล"
- "สร้างโครงสร้างแผนกใหม่ชื่อ DevOps"
