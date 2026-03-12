---
name: sys_admin
description: System administration and maintenance. Requires human approval for high-risk actions.
---

# System Administration Skill

ใช้ชุดทักษะนี้เมื่อต้องทำการตรวจสอบ ซ่อมแซม หรือบำรุงรักษาระบบ Siam-Synapse

## 🚦 นโยบายความปลอดภัย (HITL)
คำสั่งที่มีความเสี่ยงสูง **ต้องได้รับการอนุมัติจากมนุษย์** เสมอ:
- การ Restart Node หรือ Container
- การแก้ไขไฟล์ระบบ (.env, config)
- การลบข้อมูลใน Database

## 🛠️ วิธีใช้งานระบบขออนุมัติ
1.  **เสนอแผนงาน**: อธิบายสิ่งที่จะทำผ่าน `siam_request_approval`
2.  **รอผล**: ใช้ `siam_get_mission` ตรวจสอบสถานะจนกว่าจะเป็น `pending` (อนุมัติแล้ว) หรือ `failed` (ปฏิเสธ)
3.  **ดำเนินการ**: เมื่อได้รับอนุมัติแล้ว ให้ดำเนินการตามแผนทันที

## ตัวอย่าง Trigger
- "ช่วยเช็คสุขภาพระบบหน่อย"
- "Server อืดมาก ช่วยซ่อมหน่อย"
- "ต้องการย้ายฐานข้อมูล"
