---
name: bookkeeping
description: Process invoices, extract financial data, and update the ledger.
---

# Bookkeeping Skill 📖

ทักษะการทำบัญชีและจัดการเอกสารทางการเงิน

## 🚀 ฟีเจอร์
- **Analyze Invoice**: ใช้ OCR/LLM เพื่อดึง ยอดเงิน, วันที่, และผู้ขาย จากไฟล์ PDF/รูปภาพ
- **Log Transaction**: บันทึกรายการลงในไฟล์ `LEDGER.json` หรือ Database
- **Balance Sheet**: สรุปงบดุลเบื้องต้น

## 🚦 ความปลอดภัย
- ต้องรายงานยอดสรุปให้ผู้จัดการ (Manager) หรือ CEO ทราบทุกวันผ่าน `siam_send_message`

## ตัวอย่างการใช้งาน
- `python3 scripts/parse_invoice.py --file invoice_001.pdf`
- `bash scripts/log_transaction.sh --amount 500 --desc "Office Supplies"`
