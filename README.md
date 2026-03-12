# 🦞 Siam-Synapse OttoClaw Worker

> **The Sovereign AI Agent Node** — Run independent, skill-based AI agents on any Linux or Android (Termux) device.

Siam-Synapse OttoClaw is a standalone worker node designed to host agentic souls. It provides a native environment for AI agents to interact with the host system, manage files, and execute tools with full hardware access.

---

## 🚀 Fast Installation (Binary Release)

The quickest way to get started. No Go compiler required.

### 🐧 Linux (Ubuntu/Debian/CentOS)
```bash
curl -fsSL https://raw.githubusercontent.com/jkfastdevth/ottoclaw-worker/main/install.sh | sudo bash
```

### 📱 Android (Termux)
```bash
curl -fsSL https://raw.githubusercontent.com/jkfastdevth/ottoclaw-worker/main/install-termux.sh | bash
```

---

## 🛠️ How it Works

1.  **Smart Detection**: The installer identifies your CPU architecture (x64, ARM64).
2.  **Binary Download**: It fetches a pre-compiled, optimized binary from GitHub Releases.
3.  **Automatic Setup**: It configures the environment, creates necessary directories, and sets up a system service (on Linux).

---

## 📦 For Developers (Build from Source)

If you prefer to build manually or are on an unsupported platform:

```bash
git clone https://github.com/jkfastdevth/ottoclaw-worker.git
cd ottoclaw-worker
sudo bash install.sh
```

---

## 🏛️ Configuration

After installation, the worker will prompt you for:
- **MASTER_API_URL**: The address of your Siam-Synapse Master.
- **MASTER_API_KEY**: Your authentication key.
- **NODE_NAME**: A unique identifier for this node.

You can reconfigure at any time using:
```bash
ottoclaw config
```

---

## 👨‍💻 Maintainer Notes (How to Release)

To update the public binaries after making changes:

1. **Commit changes**: `git commit -am "Your update"`
2. **Push main**: `git push origin main`
3. **Tag version**: `git tag v1.0.1` (Increment as needed)
4. **Push tag**: `git push origin v1.0.1`

*The GitHub Action will automatically build and attach binaries for all platforms to the release page.*

---

## 📄 License
MIT License. Created by [jkfastdevth](https://github.com/jkfastdevth).
