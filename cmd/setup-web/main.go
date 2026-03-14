package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed index.html
var content embed.FS

type Config struct {
	MasterHost            string `json:"MASTER_HOST"`
	MasterApiKey          string `json:"MASTER_API_KEY"`
	NodeSecret            string `json:"NODE_SECRET"`
	AgentName             string `json:"AGENT_NAME"`
	OrchestratorNicknames string `json:"ORCHESTRATOR_NICKNAMES"`
	TelegramToken         string `json:"WORKER_TELEGRAM_TOKEN"`
	GoogleEmail           string `json:"GOOGLE_EMAIL"`
	GoogleAppPassword     string `json:"GOOGLE_APP_PASSWORD"`
}

var (
	logBuffer []string
	logMu     sync.Mutex
	isRunning bool
)

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/config", getConfig)
	http.HandleFunc("/api/install", startInstall)
	http.HandleFunc("/api/logs", streamLogs)

	port := "3333"
	fmt.Printf("🚀 Siam-Synapse Web Installer starting at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := content.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	// Try to find .env in project root
	cwd, _ := os.Getwd()
	envPath := filepath.Join(cwd, "..", "..", ".env")
	
	config := Config{
		MasterHost:   "192.168.1.100",
		MasterApiKey: "73e17cd67e354ad1e36259c1cea0fd974613f460427d7683e48926a34d32ec90",
		NodeSecret:   "ea710cf8c0f08298e9aa938dff0e0133",
		AgentName:   "Kaidos",
	}

	if _, err := os.Stat(envPath); err == nil {
		file, _ := os.Open(envPath)
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MASTER_HTTP_HOST=") {
				config.MasterHost = strings.TrimPrefix(line, "MASTER_HTTP_HOST=")
			} else if strings.HasPrefix(line, "MASTER_API_KEY=") {
				config.MasterApiKey = strings.TrimPrefix(line, "MASTER_API_KEY=")
			} else if strings.HasPrefix(line, "NODE_SECRET=") {
				config.NodeSecret = strings.TrimPrefix(line, "NODE_SECRET=")
			} else if strings.HasPrefix(line, "GOOGLE_EMAIL=") {
				config.GoogleEmail = strings.TrimPrefix(line, "GOOGLE_EMAIL=")
			} else if strings.HasPrefix(line, "GOOGLE_APP_PASSWORD=") {
				config.GoogleAppPassword = strings.TrimPrefix(line, "GOOGLE_APP_PASSWORD=")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func startInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if isRunning {
		http.Error(w, "Installation already in progress", http.StatusConflict)
		return
	}

	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isRunning = true
	logBuffer = []string{"[INFO] Starting installation..."}

	go runInstall(config)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Installation started"))
}

func runInstall(config Config) {
	defer func() { isRunning = false }()

	// Prepare environment variables for install.sh
	env := os.Environ()
	env = append(env, fmt.Sprintf("MASTER_HOST=%s", config.MasterHost))
	env = append(env, fmt.Sprintf("MASTER_API_KEY=%s", config.MasterApiKey))
	env = append(env, fmt.Sprintf("NODE_SECRET=%s", config.NodeSecret))
	env = append(env, fmt.Sprintf("AGENT_NAME=%s", config.AgentName))
	env = append(env, fmt.Sprintf("ORCHESTRATOR_NICKNAMES=%s", config.OrchestratorNicknames))
	env = append(env, fmt.Sprintf("WORKER_TELEGRAM_TOKEN=%s", config.TelegramToken))
	env = append(env, fmt.Sprintf("GOOGLE_EMAIL=%s", config.GoogleEmail))
	env = append(env, fmt.Sprintf("GOOGLE_APP_PASSWORD=%s", config.GoogleAppPassword))

	cwd, _ := os.Getwd()
	// Check for install.sh in standard locations
	installScript := ""
	possiblePaths := []string{
		filepath.Join(cwd, "..", "..", "install.sh"), // Standard: worker/cmd/setup-web/
		filepath.Join(cwd, "..", "install.sh"),       // Flat: setup-web/
		filepath.Join(cwd, "install.sh"),             // Local
	}

	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			installScript = p
			break
		}
	}

	// Bootstrap Mode: If install.sh is missing, try to clone the repo
	if installScript == "" {
		addLog("[INFO] Bootstrapping: install.sh not found. Preparing to pull latest source...")
		repoUrl := "https://github.com/jkfastdevth/Siam-Synapse.git"
		targetDir := filepath.Join(cwd, "workspace_source")
		
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			addLog(fmt.Sprintf("[INFO] Cloning repository: %s", repoUrl))
			cloneCmd := exec.Command("git", "clone", "--depth", "1", repoUrl, targetDir)
			if err := cloneCmd.Run(); err != nil {
				addLog(fmt.Sprintf("[ERROR] Failed to clone repository: %v", err))
				return
			}
			addLog("[SUCCESS] Source code cloned successfully.")
		} else {
			addLog("[INFO] Source directory already exists. Using existing code.")
		}
		
		installScript = filepath.Join(targetDir, "ottoclaw-worker", "install.sh")
	}

	// Execute install.sh
	cmd := exec.Command("bash", installScript)
	cmd.Env = env
	
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	multi := io.MultiReader(stdout, stderr)

	if err := cmd.Start(); err != nil {
		addLog(fmt.Sprintf("[ERROR] Failed to start: %v", err))
		return
	}

	scanner := bufio.NewScanner(multi)
	for scanner.Scan() {
		addLog(scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		addLog(fmt.Sprintf("[ERROR] Installation failed: %v", err))
	} else {
		addLog("[SUCCESS] Installation complete! You can close this tab.")
	}
}

func addLog(msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	logBuffer = append(logBuffer, msg)
	fmt.Println(msg)
}

func streamLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	lastIdx := 0
	for {
		logMu.Lock()
		if lastIdx < len(logBuffer) {
			for i := lastIdx; i < len(logBuffer); i++ {
				fmt.Fprintf(w, "data: %s\n\n", logBuffer[i])
			}
			lastIdx = len(logBuffer)
			flusher.Flush()
		}
		logMu.Unlock()

		if !isRunning && lastIdx >= len(logBuffer) && len(logBuffer) > 0 {
			// Check if finished
			if strings.Contains(logBuffer[len(logBuffer)-1], "complete") || strings.Contains(logBuffer[len(logBuffer)-1], "failed") {
				fmt.Fprintf(w, "event: end\ndata: finished\n\n")
				flusher.Flush()
				return
			}
		}

		select {
		case <-r.Context().Done():
			return
		default:
			// Just a small sleep to avoid tight loop
			// In production, use a channel for log notification
		}
	}
}
