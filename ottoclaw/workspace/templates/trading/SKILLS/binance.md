---
name: binance_trading
description: Execute trades and fetch market data from Binance / Crypto Exchanges.
---

# Binance Trading Skill 🪙

เครื่องมือสำหรับการเทรดคริปโตเคอเรนซี่ผ่าน API ของ Binance

## 🚀 ฟีเจอร์
- **Get Price**: ดึงราคาเหรียญแบบ Real-time (e.g., BTCUSDT, ETHUSDT)
- **Place Order**: ส่งคำสั่ง Buy/Sell (Limit/Market)
- **Portfolio Check**: ตรวจสอบยอดเงินคงเหลือใน Wallet

## 🚦 ความปลอดภัย
- ต้องได้รับอนุมัติจาก CEO (คุณ) ก่อนส่งคำสั่งซื้อขายที่มียอดเงิน > 100 USDT (ใช้ร่วมกับ `siam_request_approval`)

## ตัวอย่างการใช้งาน
- `python3 scripts/get_price.py --symbol BTCUSDT`
- `python3 scripts/trade.py --action buy --amount 0.1 --symbol ETHUSDT`
