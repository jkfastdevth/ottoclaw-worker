# Siam-Synapse Autonomous Health Check

ทำตามขั้นตอนนี้ทุกครั้งที่ heartbeat ทำงาน:

## 1. ดูสถานะระบบ

เรียก `siam_get_metrics` เพื่อดู CPU, node count, scaling mode

## 2. ตรวจ Sub-Agents

เรียก `siam_get_agents` เพื่อดู agents ที่ทำงานอยู่

## 3. ตัดสินใจ (เลือก 1 อย่าง)

- ถ้า **CPU > 80%** หรือ **ไม่มี worker agents** และมี node > 0:
  - เรียก `siam_get_skills` แล้ว `siam_spawn_agent` สร้าง worker ใหม่
  - รายงานว่า spawn แล้ว

- ถ้า **CPU < 20%** และมี agents > 3:
  - terminate agents ที่ไม่จำเป็น ด้วย `siam_terminate_agent`
  - รายงานว่า terminate แล้ว

- ถ้า **ระบบปกติ** (CPU อยู่ระหว่าง 20-80%, มี workers เพียงพอ):
  - ตอบ HEARTBEAT_OK เท่านั้น ห้ามส่งข้อความเพิ่ม

## กฎ

- ถ้าไม่มีอะไรต้องทำ → ตอบ **HEARTBEAT_OK** เท่านั้น (ประหยัด API quota)
- ห้าม spawn agents ซ้ำถ้ามีอยู่แล้ว
- ห้าม terminate agents ที่กำลังทำ mission สำคัญอยู่
