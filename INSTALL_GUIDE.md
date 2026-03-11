# 🦞 คู่มือการติดตั้ง OttoClaw Worker
> **Siam-Synapse Multi-Agent System — Worker Node**

---

## 📋 สารบัญ
1. [ภาพรวม](#ภาพรวม)
2. [ข้อกำหนดระบบ](#ข้อกำหนดระบบ)
3. [ติดตั้งบน Android (Termux)](#-ติดตั้งบน-android-termux)
4. [ติดตั้งบน Linux GUI / macOS / Windows](#-ติดตั้งบน-linux-gui--macos--windows)
5. [ติดตั้งบน Linux Server (Headless)](#-ติดตั้งบน-linux-server-headless)
6. [การตั้งค่าหลังติดตั้ง](#การตั้งค่าหลังติดตั้ง)
7. [คำสั่งการจัดการ](#คำสั่งการจัดการ)
8. [การอัปเดต](#การอัปเดต)
9. [การแก้ปัญหา](#การแก้ปัญหา)

---

## ภาพรวม

OttoClaw Worker ประกอบด้วย 2 ส่วนหลัก:

| Component | Binary | หน้าที่ |
|-----------|--------|---------|
| **Brain** (ottoclaw-brain) | `ottoclaw-brain` | AI Engine — รับ Mission, เรียกใช้ Tools |
| **Arm** (siam-worker) | `siam-worker` | gRPC Daemon — ส่ง Heartbeat ไปยัง Master |

```
[Master API :8080/:50051]
        │
        │ gRPC Heartbeat (siam-worker)
        ▼
    [Worker Node]
        │
        ├── siam-worker   → รับคำสั่งจาก Master, ส่งสถานะ
        └── ottoclaw-brain → AI Brain, ประมวลผล Mission
```

---

## ข้อกำหนดระบบ

| Platform | ข้อกำหนด |
|----------|---------|
| Android (Termux) | Termux จาก F-Droid, ARM64/ARMv7 |
| Linux | Ubuntu 20.04+ / Debian 11+ / Arch |
| macOS | macOS 12+ (Apple Silicon หรือ Intel) |
| Windows | Windows 10/11 + WSL2 หรือ Git Bash |

**Software ที่ต้องการ:**
- `git` — สำหรับ clone และ update code
- `Go 1.21+` — ติดตั้งอัตโนมัติ (ถ้ายังไม่มี)
- การเชื่อมต่อกับ Master Server (IP + API Key)

---

## 📱 ติดตั้งบน Android (Termux)

### ขั้นตอนที่ 1 — ติดตั้ง Termux

> **สำคัญ:** ดาวน์โหลด Termux จาก **[F-Droid](https://f-droid.org/packages/com.termux/)** เท่านั้น  
> เวอร์ชันใน Google Play Store หมดอายุแล้ว

### ขั้นตอนที่ 2 — Clone โปรเจค

```bash
# ใน Termux Terminal
pkg install git -y
git clone https://github.com/jkfastdevth/Siam-Synapse.git
cd Siam-Synapse/ottoclaw-worker
```

### ขั้นตอนที่ 3 — รัน Installer

```bash
bash install-termux.sh
```

Installer จะดำเนินการ:
1. ✅ ติดตั้ง dependencies (golang, curl) อัตโนมัติ
2. ✅ Build binary ทั้ง 2 ตัว
3. ✅ ถามชื่อ Agent และ Master Server IP
4. ✅ ถาม Telegram Bot (Optional)
5. ✅ เขียนไฟล์ config
6. ✅ เริ่ม services

### ตัวอย่างหน้าจอการตั้งค่า

```
══ Configuration Setup ══

[1/3] System
  ? Agent Name (e.g. Kaidos) [Kaidos]: Khronos
  ? Aliases (comma-sep) [Khronos]: 

  เลือกประเภท Network:
    1) Local LAN     (192.168.x.x)
    2) Tailscale VPN (100.x.x.x)
    3) VPS / Public
  
  ? Network type [1/2/3]: 1
  ? Master address [192.168.1.100]: 192.168.1.166
  ? Master API Key [***]: (กด Enter เพื่อใช้ default)
```

---

## 🖥️ ติดตั้งบน Linux GUI / macOS / Windows

### ขั้นตอนที่ 1 — Clone โปรเจค

```bash
git clone https://github.com/jkfastdevth/Siam-Synapse.git
cd Siam-Synapse/ottoclaw-worker
```

### ขั้นตอนที่ 2 — รัน GUI Installer

```bash
# Linux / macOS
bash install-gui.sh

# Windows (WSL2 หรือ Git Bash)
bash install-gui.sh
```

### GUI Dialogs ตาม OS

| OS | GUI Toolkit | ที่ติดตั้ง |
|-----|------------|-----------|
| Linux (GNOME) | Zenity | ติดตั้งอัตโนมัติ |
| Linux (KDE) | kdialog | ใช้ที่มีอยู่แล้ว |
| macOS | AppleScript | built-in |
| Windows WSL | PowerShell InputBox | built-in |
| (ไม่มี GUI) | Terminal prompts | fallback อัตโนมัติ |

> 💡 **Tip:** ถ้าไม่มี GUI toolkit จะ fallback เป็น terminal prompts อัตโนมัติ ใช้งานได้ปกติ

### Services ที่ติดตั้งตาม OS

| OS | Service Manager | Auto-Start |
|-----|----------------|-----------|
| Linux | systemd | ✅ Boot |
| macOS | launchd | ✅ Login |
| Windows | Task Scheduler | ✅ Login |

---

## 🐧 ติดตั้งบน Linux Server (Headless)

สำหรับ Server ที่ไม่มี GUI (VPS, Raspberry Pi):

```bash
git clone https://github.com/jkfastdevth/Siam-Synapse.git
cd Siam-Synapse/ottoclaw-worker
sudo bash install.sh
```

Installer จะใช้ **terminal prompts** และติดตั้งเป็น **systemd service** อัตโนมัติ

---

## การตั้งค่าหลังติดตั้ง

### ข้อมูลที่ต้องเตรียม

| ข้อมูล | ตัวอย่าง | หาได้จาก |
|--------|---------|---------|
| Agent Name | `Kaidos` | ตั้งชื่อเอง |
| Master IP | `192.168.1.166` | เครื่องที่รัน Master API |
| Master API Key | `73e17cd67...` | `/etc/siam-synapse/.env` บน Master |
| Node Secret | `ea710cf8...` | `.env` บน Master |
| Telegram Token | `12345:ABC...` | @BotFather (Optional) |

### ไฟล์ Config หลัก

| Platform | ที่อยู่ |
|----------|--------|
| Linux (systemd) | `/etc/ottoclaw/env` |
| macOS / Windows | `~/.ottoclaw/env` |
| Android (Termux) | `~/.ottoclaw/env` |

---

## คำสั่งการจัดการ

### Android (Termux)

```bash
ottoclaw start           # เริ่ม Brain + Arm ใน background
ottoclaw stop            # หยุด
ottoclaw restart         # รีสตาร์ท
ottoclaw status          # ดูสถานะ (Running / Stopped)
ottoclaw log brain       # ดู Brain Log แบบ real-time
ottoclaw log arm         # ดู Arm Log แบบ real-time
ottoclaw config          # ตั้งค่าใหม่
ottoclaw update          # อัปเดต code และ rebuild
ottoclaw uninstall       # ลบออก
```

### Linux (systemd)

```bash
# Service Management
sudo systemctl start ottoclaw-worker      # เริ่ม
sudo systemctl stop ottoclaw-worker       # หยุด
sudo systemctl restart ottoclaw-worker    # รีสตาร์ท
sudo systemctl status ottoclaw-worker     # ดูสถานะ

# Logs
journalctl -u ottoclaw-worker -f          # Brain log (real-time)
journalctl -u siam-worker -f              # Arm log (real-time)

# Management
sudo ottoclaw config                       # ตั้งค่าใหม่
sudo ottoclaw update                       # อัปเดต + rebuild
sudo ottoclaw uninstall                    # ลบออก
```

### macOS

```bash
# Service Management (launchd)
launchctl list | grep siam                                  # ดูสถานะ
launchctl stop com.siam-synapse.ottoclaw-brain              # หยุด Brain
launchctl start com.siam-synapse.ottoclaw-brain             # เริ่ม Brain

# Logs
tail -f ~/.ottoclaw/logs/ottoclaw-brain.log                 # Brain log
tail -f ~/.ottoclaw/logs/siam-worker.log                    # Arm log

# Management
bash install-gui.sh update                                  # อัปเดต
bash install-gui.sh                                         # ตั้งค่าใหม่
```

### Windows (WSL / Git Bash)

```bash
# Start / Stop
~/.ottoclaw/start.bat      # เริ่มทำงาน
~/.ottoclaw/stop.bat       # หยุดทำงาน

# Logs
tail -f ~/.ottoclaw/logs/ottoclaw-brain.log

# Update
bash install-gui.sh update
```

---

## การอัปเดต

### ทุก Platform — อัปเดต 1 คำสั่ง

| Platform | คำสั่ง |
|----------|--------|
| Android (Termux) | `ottoclaw update` |
| Linux (systemd) | `sudo ottoclaw update` |
| macOS / Windows | `bash install-gui.sh update` |

**สิ่งที่ `update` ทำ:**
1. `git pull --ff-only` — ดึง code ล่าสุด
2. หยุด services
3. `go build` rebuild binary ทั้ง 2 ตัว
4. รีสตาร์ท services อัตโนมัติ

> ⚠️ ถ้า `git pull` ล้มเหลว (มี conflicts) ต้อง resolve manually ก่อน แล้วค่อย update ใหม่

---

## การแก้ปัญหา

### ❌ ปัญหา: WebSocket ไม่ต่อ / Agent ไม่ขึ้นใน Dashboard

**ตรวจสอบ:**
1. ยืนยัน Master IP ถูกต้อง (`ottoclaw config`)
2. ตรวจสอบว่า Master API พอร์ต `8080` และ `50051` เปิดอยู่
3. ดู log: `ottoclaw log arm` หรือ `journalctl -u siam-worker`

### ❌ ปัญหา: `go build` ล้มเหลว

```bash
# ตรวจสอบ Go version
go version

# ถ้า Go ไม่ได้อยู่ใน PATH
export PATH="/usr/local/go/bin:$PATH"
# หรือบน Termux:
export PATH="$PREFIX/bin:$PATH"
```

### ❌ ปัญหา: Agent ไม่รู้จักตัวเอง (Soul ไม่โหลด)

```bash
# ดูไฟล์ SOUL.md
cat ~/.ottoclaw/workspace/v2/SOUL.md      # macOS/Windows/Termux
cat /var/lib/ottoclaw/workspace/v2/SOUL.md # Linux systemd

# Re-forge soul ด้วย config ใหม่
ottoclaw config       # หรือ sudo ottoclaw config
```

### ❌ ปัญหา: Port 50051 ไม่ตอบสนอง

ตรวจสอบ firewall บน Master Server:
```bash
# Ubuntu/Debian
sudo ufw allow 50051/tcp
sudo ufw allow 8080/tcp

# ทดสอบ connectivity
nc -zv <MASTER_IP> 50051
nc -zv <MASTER_IP> 8080
```

### ❌ ปัญหา: Telegram Bot 409 Conflict

เกิดจาก Bot Token ถูกใช้โดยหลาย instance:
```bash
# แก้: ตรวจสอบว่า Token นี้ใช้กับ Orchestrator เท่านั้น
# Worker ไม่ควรใช้ TELEGRAM_BOT_TOKEN เดียวกับ Orchestrator
ottoclaw config    # เปลี่ยน Token หรือ ปล่อยว่าง
```

---

## 📁 โครงสร้างไฟล์

```
~/.ottoclaw/                    # หรือ /var/lib/ottoclaw บน Linux
├── env                         # Environment config (600)
├── config.json                 # ottoclaw-brain config
├── workspace/
│   └── v2/
│       ├── SOUL.md             # Agent Identity (AI Persona)
│       ├── SOUL_ID             # Agent Name (สำหรับ recovery)
│       ├── NODE_ID             # Node identifier
│       └── TOOLS               # [Phase 2] รายชื่อ tool ที่ active
└── logs/
    ├── ottoclaw-brain.log      # Brain log (Termux/macOS/Windows)
    └── siam-worker.log         # Arm log
```

---

## 🔗 ลิงก์ที่เกี่ยวข้อง

- **Master API Docs:** `http://<MASTER_IP>:8080/api/agent/v1/`
- **Dashboard:** `http://<MASTER_IP>:8080`
- **Termux F-Droid:** https://f-droid.org/packages/com.termux/
- **Go Download:** https://go.dev/dl/
