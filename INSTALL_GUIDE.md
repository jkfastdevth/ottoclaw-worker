# 🦞 คู่มือการติดตั้ง OttoClaw Worker
> **Siam-Synapse Multi-Agent System — Worker Node**

---

## 📋 สารบัญ
1. [ภาพรวม](#ภาพรวม)
2. [ข้อกำหนดระบบ](#ข้อกำหนดระบบ)
3. [วิธีที่ 1: Web-Based Installer (แนะนำ ✨)](#วิธีที่-1-web-based-installer-แนะนำ-)
4. [วิธีที่ 2: Linux GUI / macOS / Windows](#วิธีที่-2-linux-gui--macos--windows)
5. [วิธีที่ 3: Linux Server (Headless/CLI)](#วิธีที่-3-linux-server-headlesscli)
6. [วิธีที่ 4: Android (Termux)](#วิธีที่-4-android-termux)
7. [การตั้งค่าหลังติดตั้ง](#การตั้งค่าหลังติดตั้ง)
8. [การตั้งค่า Google Skill (Gmail/Drive)](#การตั้งค่า-google-skill-gmaildrive)
9. [คำสั่งการจัดการ](#คำสั่งการจัดการ)
10. [การอัปเดต](#การอัปเดต)
11. [การแก้ปัญหา](#การแก้ปัญหา)

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

## 🌐 วิธีที่ 1: Web-Based Installer (แนะนำ ✨)

นี่เป็นวิธีที่ง่ายและเป็นมืออาชีพที่สุด โดยจะเปิดหน้าเว็บสวยงามให้คุณตั้งค่าผ่าน Browser พร้อมแสดง Real-time Terminal Log ขณะติดตั้ง

### ขั้นตอนการใช้งาน:

```bash
# Clone เฉพาะ Folder ติดตั้ง (หรือ Clone ทั้งโปรเจกต์)
git clone https://github.com/jkfastdevth/Siam-Synapse.git
cd Siam-Synapse/ottoclaw-worker

# รันตัวติดตั้งเว็บ
sudo bash install-web.sh
```

### ฟีเจอร์เด่น:
- **Bootstrap Mode**: หากคุณมีเพียงชุดไฟล์ติดตั้ง ระบบจะทำการดึง Source Code ที่เหลือจาก GitHub มาให้เองอัตโนมัติ
- **Premium UI**: หน้าจอตั้งค่าแบบ Glassmorphism ทันสมัย
- **Auto-Injection**: หากพบไฟล์ `.env` ระบบจะดึง `MASTER_API_KEY` และ `NODE_SECRET` มาเติมให้ทันที
- **Deployment Console**: เห็นทุกขั้นตอนการ Build และลง Service แบบสดๆ บนหน้าเว็บที่ `http://localhost:3333`

---

## 🖥️ วิธีที่ 2: Linux GUI / macOS / Windows

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

## 🐧 วิธีที่ 3: Linux Server (Headless/CLI)

สำหรับ Server ที่ไม่มี GUI (VPS, Raspberry Pi):

```bash
git clone https://github.com/jkfastdevth/Siam-Synapse.git
cd Siam-Synapse/ottoclaw-worker
sudo bash install.sh
```

Installer จะใช้ **terminal prompts** และติดตั้งเป็น **systemd service** อัตโนมัติ

> 💡 **Tip:** หน้าจอ CLI จะใช้ชื่อตัวแปรเทคนิค (e.g., `MASTER_API_KEY`) เพื่อให้ตรงกับมาตรฐานสากล

---

## 📱 วิธีที่ 4: Android (Termux)

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

## 📧 การตั้งค่า Google Skill (Gmail/Drive)

Siam-Synapse รองรับการทำงานร่วมกับ Google Services (Gmail, Calendar, Drive) แบบเครื่องเดียวหรือแยกบัญชีรายบุคคล

### 1. การเตรียมข้อมูลจาก Google
*   **Gmail / Calendar**: เปิดใช้งาน [2-Step Verification](https://myaccount.google.com/signinoptions/two-step-verification) และสร้าง **[App Password](https://myaccount.google.com/apppasswords)** (16 หลัก) เพื่อใช้แทนรหัสผ่านปกติ
*   **Google Drive**: แนะนำให้ใช้ **Service Account** (ชุดกุญแจ JSON) เพื่อความเป็นส่วนตัวและความปลอดภัยสูง

### 2. การตั้งค่า Credentials
คุณสามารถใส่ข้อมูลได้ 2 ระดับ:

*   **Global Level (พนักงานทุกคนใช้บัญชีเดียวกัน)**:
    ใส่ในไฟล์ `/etc/ottoclaw/env` (หรือ `~/.ottoclaw/env`):
    ```env
    GOOGLE_EMAIL=admin@your-company.com
    GOOGLE_APP_PASSWORD=abcd-efgh-ijkl-mnop
    ```
*   **Agent Level (พนักงานแต่ละคนมีบัญชีแยกกัน)**:
    สร้างไฟล์ชื่อ `env` ในโฟลเดอร์ Workspace ของ Agent นั้นๆ:
    ```bash
    # ตัวอย่าง: /var/lib/ottoclaw/workspace/workspace-hr/env
    GOOGLE_EMAIL=hr.agent@your-company.com
    GOOGLE_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx
    ```

### 3. ชุดเครื่องมือที่มีให้ใช้งาน
เมื่อตั้งค่าถูกต้อง Agent จะสามารถใช้เครื่องมือเหล่านี้ได้ทันที:
*   **Gmail**: `siam_send_email`, `siam_read_emails` (ใช้ IMAP ดึงข้อมูล)
*   **Calendar**: `siam_list_calendar`, `siam_create_calendar_event`
*   **Drive**: `siam_drive_upload`, `siam_drive_search`, `siam_drive_download`

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
