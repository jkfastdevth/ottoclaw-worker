// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"time"
// 	"net"
// 	 //"io"
// 	"os"
// 	"os/exec"
// 	"github.com/shirou/gopsutil/v3/cpu"
// 	"github.com/shirou/gopsutil/v3/mem"
// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/credentials/insecure"
// 	"github.com/jkfastdevth/Siam-Synapse/proto"
// )

// type workerServer struct {
// 	proto.UnimplementedMasterControlServer
// 	nodeID string
// }

// func restartContainer(containerName string) error {
// 	log.Printf("🐳 Executing: docker restart %s", containerName)
// 	cmd := exec.Command("docker", "restart", containerName)
// 	return cmd.Run()
// }

// // Implement ManageContainer ใน Worker
// func (s *workerServer) ManageContainer(ctx context.Context, req *proto.ContainerAction) (*proto.Ack, error) {
// 	log.Printf("🛠️ Received Container Action: %s on %s", req.Action, req.ContainerName)

// 	if req.Action == "restart" {
// 		err := restartContainer(req.ContainerName)
// 		if err != nil {
// 			log.Printf("❌ Failed to restart container: %v", err)
// 			return &proto.Ack{Success: false, Message: err.Error()}, nil
// 		}
// 		log.Printf("✅ Container %s restarted successfully", req.ContainerName)
// 		return &proto.Ack{Success: true, Message: "Container restarted"}, nil
// 	}

// 	return &proto.Ack{Success: false, Message: "Unknown action"}, nil
// }

// // 1. ฟังก์ชันหาชื่อ Container ตัวเอง
// func getContainerName() string {
//     // ใช้ hostname ของ container มักจะเป็น ID หรือชื่อที่ Docker ตั้งให้
// 	hostname, err := os.Hostname()
// 	if err != nil {
// 		return "unknown"
// 	}
// 	return hostname
// }

// func main() {

// 	conn, err := grpc.Dial("master:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	client := proto.NewMasterControlClient(conn)

// 	if err != nil {
// 		log.Fatalf("❌ Could not connect to Master: %v", err)
// 	}
// 	defer conn.Close()

// 	stream, err := client.GetCommand(context.Background(), &proto.NodeStatus{NodeId: "worker-ubuntu-01"})
//     if err != nil {
//         log.Fatalf("❌ Failed to get command stream: %v", err)
//     }
//     log.Println("📡 Stream opened")

// 	go func() {
// 			lis, err := net.Listen("tcp", ":50052")
// 			if err != nil {
// 				log.Fatalf("failed to listen: %v", err)
// 			}
// 			s := grpc.NewServer()
// 			proto.RegisterMasterControlServer(s, &workerServer{
// 				nodeID: os.Getenv("NODE_ID"), // ใช้ nodeID จาก env
// 			})
// 			log.Println("🛠️ Worker gRPC Server listening on :50052")
// 			if err := s.Serve(lis); err != nil {
// 				log.Fatalf("failed to serve: %v", err)
// 			}
// 		}()

// 	// 1. เชื่อมต่อกับ Master (Port 50051)
// 	// ใน Docker Compose ให้ใช้ชื่อ service "master" แทน IP

// 	grpcClient := proto.NewMasterControlClient(conn)

// 	fmt.Println("🚀 Worker Node Started: Sending heartbeats to Master...")

// // 2. จำลอง Node ID
// 	nodeID := os.Getenv("NODE_ID")
// 	if nodeID == "" {
// 		nodeID = "worker-ubuntu-01"
// 	}
// 	// 3. --- เพิ่มส่วนนี้ ---
// 	// คอยฟังคำสั่ง (Stream)
// 	stream, err := grpcClient.GetCommand(newGRPCCtx(), &proto.NodeStatus{NodeId: nodeID})
// 	if err != nil {
// 		log.Fatalf("could not get command: %v", err)
// 	}

// 	go func() {
// 		for {
// 			cmd, err := stream.Recv()
// 		if err != nil {
//             log.Printf("❌ Error receiving command: %v", err)
//             break
//         }
// 		fmt.Printf("📥 Received Command: %s\n", cmd.Command)
// 			// if err != nil {
// 			// 	log.Fatalf("Error receiving command: %v", err)
// 			// }

// 			// fmt.Printf("📥 Received Command: %s\n", cmd.Command)
// 			if cmd.Command == "restart" {
//             // ใช้ชื่อจริงที่คุณตั้งใน docker-compose: sworker-ubuntu-01
//             err := restartContainer("sworker-ubuntu-01")
//            if cmd.Command == "restart" {
//                 restartContainer("sworker-ubuntu-01")
//             }
//         }
// 			// ตรงนี้คือ Logic ที่จะจัดการคำสั่ง เช่นสั่ง restart หรือ stop
// 		}
// 	}()
// 	// -----------------

// 	// 2. Loop ส่ง Heartbeat ทุกๆ 5 วินาที
// 	for {
// 		// ดึงข้อมูลระบบจริง
// 		c, _ := cpu.Percent(0, false)
// 		m, _ := mem.VirtualMemory()

// 		status := &proto.NodeStatus{
// 			NodeId:   "worker-ubuntu-01",
// 			CpuUsage: float32(c[0]),
// 			RamUsage: float32(m.UsedPercent),
// 			Status:   "Online",
// 		}

// 		// ยิง gRPC ไปที่ Master
// 		res, err := grpcClient.ReportStatus(context.Background(), status)
// 		if err != nil {
// 			fmt.Printf("⚠️ Error sending status: %v\n", err)
// 		} else {
// 			fmt.Printf("✅ Master Response: %s (CPU: %.1f%%)\n", res.Message, status.CpuUsage)
// 		}

// 		time.Sleep(5 * time.Second)
// 	}
// }

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"bytes"
	"encoding/json"
	"sync/atomic"
	"net/http"
	"os/exec"
	"syscall"

	"github.com/jkfastdevth/Siam-Synapse/proto" // เปลี่ยนเป็น path โปรเจคของคุณ
	"github.com/shirou/gopsutil/cpu"            // ต้องติดตั้งเพิ่ม: go get github.com/shirou/gopsutil/cpu
	"github.com/shirou/gopsutil/mem"            // ต้องติดตั้งเพิ่ม: go get github.com/shirou/gopsutil/mem
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"regexp"
)

// isTermux returns true when running inside Android Termux environment.
func isTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" || strings.Contains(os.Getenv("PREFIX"), "com.termux")
}

// recordWAV records audio to wavPath for durSec seconds.
// Tries: arecord → ffmpeg(alsa) → ffmpeg(pulse) → sox → parec → pw-record
func recordWAV(ctx context.Context, wavPath, durSec string) error {
	if isTermux() {
		return exec.CommandContext(ctx, "termux-microphone-record",
			"-e", "WAV", "-l", durSec, "-f", wavPath).Run()
	}
	try := [][]string{
		{"arecord", "-d", durSec, "-f", "S16_LE", "-r", "16000", "-c", "1", wavPath},
		{"ffmpeg", "-y", "-f", "alsa", "-i", "default", "-t", durSec, "-ar", "16000", "-ac", "1", wavPath},
		{"ffmpeg", "-y", "-f", "pulse", "-i", "default", "-t", durSec, "-ar", "16000", "-ac", "1", wavPath},
		{"sox", "-t", "alsa", "default", "-r", "16000", "-c", "1", "-e", "signed-integer", "-b", "16", wavPath, "trim", "0", durSec},
		{"parec", "--file-format=wav", "--rate=16000", "--channels=1", wavPath},
		{"pw-record", "--target", "alsa_input.default", wavPath},
	}
	var lastErr error
	for _, args := range try {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// listenLoops tracks active SYSTEM_LISTEN_LOOP goroutines by their loopID.
var (
	listenLoops   = map[string]context.CancelFunc{}
	listenLoopsMu sync.Mutex
)

// Phase 5.4: isSpeaking tracks whether TTS is currently playing.
// Set to true during SYSTEM_SPEAK, checked in SYSTEM_LISTEN to prevent echo.
var isSpeaking atomic.Bool

// voicePrintsDir returns the directory for stored resemblyzer voice embeddings.
func voicePrintsDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".picoclaw", "voice_prints")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// masterHTTPBase returns the base HTTP URL of the master server.
// Derived from MASTER_HTTP_URL env or converted from MASTER_GRPC_URL (port 50051→3000).
func masterHTTPBase() string {
	if u := os.Getenv("MASTER_HTTP_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	grpcURL := os.Getenv("MASTER_GRPC_URL")
	if grpcURL == "" {
		grpcURL = "master:50051"
	}
	// Strip port, use HTTP port 3000
	host := grpcURL
	if idx := strings.LastIndex(grpcURL, ":"); idx >= 0 {
		host = grpcURL[:idx]
	}
	return "http://" + host + ":3000"
}

// uploadVoicePrint uploads a .npy embedding file to master central storage.
func uploadVoicePrint(name, npyPath string) error {
	data, err := os.ReadFile(npyPath)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	body := fmt.Sprintf(`{"name":%q,"data":%q}`, name, encoded)
	apiKey := os.Getenv("MASTER_API_KEY")
	req, err := http.NewRequest("POST", masterHTTPBase()+"/api/voice-prints",
		strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed: %s", resp.Status)
	}
	return nil
}

// vpSyncedAt tracks last voice prints sync time (avoid repeated fetches).
var (
	vpSyncedAt   time.Time
	vpSyncMu     sync.Mutex
	vpSyncTTL    = 5 * time.Minute
)

// syncVoicePrints fetches voice prints from master and saves to local voicePrintsDir.
// Uses a 5-minute cache to avoid fetching on every SYSTEM_LISTEN call.
func syncVoicePrints() {
	vpSyncMu.Lock()
	defer vpSyncMu.Unlock()
	if time.Since(vpSyncedAt) < vpSyncTTL {
		return
	}
	apiKey := os.Getenv("MASTER_API_KEY")
	req, err := http.NewRequest("GET", masterHTTPBase()+"/api/voice-prints", nil)
	if err != nil {
		return
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("⚠️ [VoicePrint] sync failed: %v", err)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Prints []struct {
			Name string `json:"name"`
			Data string `json:"data"`
		} `json:"prints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}
	vpDir := voicePrintsDir()
	count := 0
	for _, p := range result.Prints {
		raw, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			continue
		}
		dest := filepath.Join(vpDir, p.Name+".npy")
		if os.WriteFile(dest, raw, 0o644) == nil {
			count++
		}
	}
	if count > 0 {
		log.Printf("🎙️ [VoicePrint] synced %d prints from master", count)
	}
	vpSyncedAt = time.Now()
}

// checkVoicePrintExists returns true if a voice print for the given name exists on master.
func checkVoicePrintExists(name string) bool {
	apiKey := os.Getenv("MASTER_API_KEY")
	req, err := http.NewRequest("GET", masterHTTPBase()+"/api/voice-prints", nil)
	if err != nil {
		return false
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Prints []struct {
			Name string `json:"name"`
		} `json:"prints"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return false
	}
	for _, p := range result.Prints {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

// autoEnrollSelfVoice generates a TTS sample for this worker's agent identity
// and uploads the voice print to master if not already enrolled.
// This ensures every agent's voice is automatically known to the speaker ID system.
func autoEnrollSelfVoice(ctx context.Context) {
	currentSoulMu.RLock()
	agentName := strings.ToLower(strings.TrimSpace(currentSoul))
	currentSoulMu.RUnlock()
	if agentName == "" {
		return
	}
	// Only enroll known agent names (not arbitrary node IDs)
	knownAgents := map[string]bool{"auric": true, "kaidos": true, "kook": true}
	if !knownAgents[agentName] {
		// Allow any agent that has a proper name (not numeric node ID)
		if len(agentName) < 3 || agentName[0] >= '0' && agentName[0] <= '9' {
			return
		}
	}

	// Skip if already enrolled
	if checkVoicePrintExists(agentName) {
		log.Printf("🎙️ [AutoEnroll] voice print for '%s' already exists", agentName)
		return
	}

	log.Printf("🎙️ [AutoEnroll] generating voice sample for agent: %s", agentName)

	// Generate TTS sample using edge-tts or espeak
	sampleText := fmt.Sprintf("สวัสดี ผมคือ %s พร้อมรับคำสั่งแล้ว", agentName)
	tmpWav := fmt.Sprintf("/tmp/enroll-auto-%s.wav", agentName)
	tmpMp3 := fmt.Sprintf("/tmp/enroll-auto-%s.mp3", agentName)
	defer os.Remove(tmpWav)
	defer os.Remove(tmpMp3)

	generated := false
	// Try edge-tts → convert to WAV
	if exec.CommandContext(ctx, "edge-tts",
		"--voice", "th-TH-PremwadeeNeural",
		"--text", sampleText,
		"--write-media", tmpMp3).Run() == nil {
		if exec.CommandContext(ctx, "ffmpeg", "-y", "-i", tmpMp3,
			"-ar", "16000", "-ac", "1", tmpWav).Run() == nil {
			generated = true
		}
	}
	// Fallback: espeak-ng
	if !generated {
		if exec.CommandContext(ctx, "espeak-ng", "-v", "th", "-w", tmpWav, sampleText).Run() == nil {
			generated = true
		} else if exec.CommandContext(ctx, "espeak-ng", "-w", tmpWav, sampleText).Run() == nil {
			generated = true
		}
	}
	if !generated {
		log.Printf("⚠️ [AutoEnroll] TTS generation failed for %s", agentName)
		return
	}

	// Generate embedding with resemblyzer
	vpDir := voicePrintsDir()
	embedPath := filepath.Join(vpDir, agentName+".npy")
	pyEnroll := `import sys, numpy as np
from resemblyzer import VoiceEncoder, preprocess_wav
from pathlib import Path
enc = VoiceEncoder()
wav = preprocess_wav(Path(sys.argv[1]))
embed = enc.embed_utterance(wav)
np.save(sys.argv[2], embed)
print("ok")
`
	out, err := exec.CommandContext(ctx, "python3", "-c", pyEnroll, tmpWav, embedPath).Output()
	if err != nil {
		log.Printf("⚠️ [AutoEnroll] resemblyzer failed for %s: %v", agentName, err)
		return
	}
	log.Printf("🎙️ [AutoEnroll] embedding generated: %s", strings.TrimSpace(string(out)))

	// Upload to master
	if err := uploadVoicePrint(agentName, embedPath); err != nil {
		log.Printf("⚠️ [AutoEnroll] upload failed for %s: %v", agentName, err)
		return
	}
	vpSyncedAt = time.Time{} // invalidate cache
	log.Printf("✅ [AutoEnroll] voice print for '%s' enrolled and uploaded", agentName)
}

// guardCommand blocks dangerous shell command patterns before execution.
func guardCommand(cmd string) error {
	lower := strings.ToLower(cmd)
	blocked := []string{
		"rm -rf /", "rm -rf /*", "rm -rf ~",
		"> /dev/sda", "dd if=", "mkfs", "fdisk",
		"shutdown", "reboot", "halt", "poweroff",
		"eval ", "> /etc/passwd", "> /etc/shadow",
		"chmod 777 /", "chmod -r /", "chown root /",
		":(){ :|:& };:", // fork bomb
	}
	for _, pattern := range blocked {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("blocked dangerous pattern: %q", pattern)
		}
	}
	if strings.Contains(cmd, "../../../") {
		return fmt.Errorf("path traversal detected in command")
	}
	return nil
}

// sanitizeFilename ensures a workspace filename cannot escape the workspace directory.
func sanitizeFilename(name string) error {
	if strings.Contains(name, "..") {
		return fmt.Errorf("'..' not allowed in filename")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute path not allowed in filename")
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return fmt.Errorf("unsafe character %q in filename", c)
		}
	}
	return nil
}

// backoffDuration returns exponential backoff duration capped at 60s.
// attempt=0 → 1s, 1 → 2s, 2 → 4s, 3 → 8s, 4 → 16s, 5+ → 60s
// detectPlatform returns a canonical platform string for this node.
// Used to tag Evolution rules with platform context.
func detectPlatform() string {
	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		return "android-termux"
	}
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func backoffDuration(attempt int) time.Duration {
	if attempt > 5 {
		attempt = 5
	}
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

func NormalizeID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	// Replace non-alphanumeric with dashes
	reg := regexp.MustCompile(`[^a-z0-9_-]+`)
	result := reg.ReplaceAllString(lower, "-")
	// Trim leading/trailing dashes
	result = strings.Trim(result, "-")
	return result
}

var (
	// 🛡️ Brain & Soul Management
	brainMutex    sync.Mutex
	currentBrain  *exec.Cmd
	currentSoul   string
	currentSoulMu sync.RWMutex
)

// getSafeEnv returns an allowlisted environment for the brain process.
// Only explicitly permitted variables are passed to prevent leaking infrastructure
// credentials (gRPC keys, DB URLs, etc.) into user-controlled brain processes.
func getSafeEnv() []string {
	isOrchestrator := (os.Getenv("OTTOCLAW_MODE") == "orchestrator")

	// Allowlist of env var prefixes/names safe to pass to the brain
	allowPrefixes := []string{
		"OTTOCLAW_",
		"AGENT_",
		"LLM_",
		"ANTHROPIC_",
		"OPENAI_",
		"OLLAMA_",
		"HOME",
		"PATH",
		"TERM",
		"LANG",
		"TZ",
		"NODE_ID",
		"ORCHESTRATOR_TELEGRAM_TOKEN",
		"TELEGRAM_",
		"LINE_CHANNEL_ACCESS_TOKEN",
		"GOOGLE_",
		"SIAM_",
		"MASTER_API_",
		"MASTER_",
		"GEMINI_",
	}

	safeEnv := make([]string, 0, 32)
	for _, e := range os.Environ() {
		for _, prefix := range allowPrefixes {
			if strings.HasPrefix(e, prefix) {
				safeEnv = append(safeEnv, e)
				break
			}
		}
	}

	// 🛡️ Disable Telegram Polling for workers to prevent 409 Conflict
	if !isOrchestrator {
		safeEnv = append(safeEnv, "OTTOCLAW_CHANNELS_TELEGRAM_ENABLED=false")
	}

	return safeEnv
}

// getBatteryAndTemp reads hardware telemetry from sysfs (Android/Linux)
func getBatteryAndTemp() (float32, float32) {
	battery := float32(0)
	temp := float32(0)

	// Try battery paths
	batPaths := []string{
		"/sys/class/power_supply/battery/capacity",
		"/sys/class/power_supply/BAT0/capacity",
	}
	for _, path := range batPaths {
		if data, err := os.ReadFile(path); err == nil {
			if val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 32); err == nil {
				battery = float32(val)
				break
			}
		}
	}

	// Try thermal paths (results are usually in millidegrees Celsius)
	tempPaths := []string{
		"/sys/class/thermal/thermal_zone0/temp",
		"/sys/class/thermal/thermal_zone1/temp",
	}
	for _, path := range tempPaths {
		if data, err := os.ReadFile(path); err == nil {
			if val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 32); err == nil {
				temp = float32(val) / 1000.0
				break
			}
		}
	}

	return battery, temp
}

// restartContainer restarts a Docker container by name with a 30s timeout.
// containerName is validated to prevent shell injection.
func restartContainer(containerName string) error {
	if containerName == "" {
		return fmt.Errorf("container name is empty")
	}
	// Validate: only alphanumeric, dash, underscore, dot allowed
	for _, c := range containerName {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return fmt.Errorf("invalid character %q in container name", c)
		}
	}
	log.Printf("🐳 Restarting container: %s", containerName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "restart", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker restart failed: %v — %s", err, string(output))
	}
	log.Printf("✅ Container %s restarted successfully", containerName)
	return nil
}

// siamArmConfig is a minimal config struct used to read node_secret from ~/.ottoclaw/config.json.
type siamArmConfig struct {
	Channels struct {
		SiamSync struct {
			NodeSecret string `json:"node_secret"`
			APIKey     string `json:"api_key"`
		} `json:"siam_sync"`
	} `json:"channels"`
}

// loadNodeSecretFromConfig reads node_secret from ~/.ottoclaw/config.json.
func loadNodeSecretFromConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".ottoclaw", "config.json"))
	if err != nil {
		return ""
	}
	var cfg siamArmConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Channels.SiamSync.NodeSecret
}

// resolveNodeSecret returns the current node secret, re-reading config.json on every call
// so that updates pushed by the brain (via heartbeat config_patch) take effect without restart.
func resolveNodeSecret() string {
	if s := os.Getenv("NODE_SECRET"); s != "" {
		return s
	}
	if s := loadNodeSecretFromConfig(); s != "" {
		return s
	}
	return os.Getenv("MASTER_API_KEY")
}

func main() {
	// newGRPCCtx returns a context with auth metadata attached.
	// Calls resolveNodeSecret() each time so config.json changes take effect immediately.
	newGRPCCtx := func() context.Context {
		secret := resolveNodeSecret()
		if secret == "" {
			return context.Background()
		}
		return metadata.AppendToOutgoingContext(context.Background(), "x-node-secret", secret)
	}

	// 1. จำลอง Node ID จาก ENV หรือสร้างจาก Hostname (Dynamic Identity)
	workspaceDir := os.Getenv("OTTOCLAW_WORKSPACE")
	if workspaceDir == "" {
		workspaceDir = "/app/workspace"
	}
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		log.Printf("⚠️  Failed to ensure %s exists: %v", workspaceDir, err)
	}

	nodeIDPath := filepath.Join(workspaceDir, "NODE_ID")
	nodeID := os.Getenv("NODE_ID")

	if nodeID == "" {
		// Try to load from disk first
		savedID, err := os.ReadFile(nodeIDPath)
		if err == nil && len(savedID) > 0 {
			nodeID = strings.TrimSpace(string(savedID))
			log.Printf("🆔 [Identity] Restored persistent identity from disk: %s", nodeID)
		} else {
			// Generate new one
			hostname, err := os.Hostname()
			if err != nil || hostname == "" || hostname == "localhost" {
				hostname = "vessel"
			}
			suffix := time.Now().Unix() % 10000
			nodeID = fmt.Sprintf("%s-%04d", hostname, suffix)
			log.Printf("🆔 [Identity] Generated new dynamic identity: %s", nodeID)

			// Save to disk for next time
			if err := os.WriteFile(nodeIDPath, []byte(nodeID), 0644); err != nil {
				log.Printf("⚠️  Failed to persist NODE_ID: %v", err)
			}
		}
	}

	// 🛡️ Soul Identity Loading:
	// 1. Prioritize AGENT_NAME env (Force Pinning e.g. for Orchestrator)
	// 2. Fallback to Soul Recovery from disk (Persistence for BareMetal nodes)
	initialSoul := os.Getenv("AGENT_NAME")
	if initialSoul != "" {
		currentSoulMu.Lock()
		currentSoul = initialSoul
		currentSoulMu.Unlock()
		log.Printf("👑 [Soul Pinning] Identity pinned via environment: %s", initialSoul)
	}

	soulIDPath := filepath.Join(workspaceDir, "SOUL_ID")
	if initialSoul == "" {
		if savedSoul, err := os.ReadFile(soulIDPath); err == nil && len(savedSoul) > 0 {
			recoveredSoul := strings.TrimSpace(string(savedSoul))
			currentSoulMu.Lock()
			currentSoul = recoveredSoul
			currentSoulMu.Unlock()
			log.Printf("✨ [Soul Recovery] Restored existing soul identity: %s", recoveredSoul)
		}
	}

	// 🚀 Auto-Start Brain if identity is established (Master restart/Node reboot resilience)
	currentSoulMu.RLock()
	activeIdentity := currentSoul
	currentSoulMu.RUnlock()

	// สร้าง Context หลักของ Worker (ก่อน auto-start brain)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	if activeIdentity != "" && os.Getenv("OTTOCLAW_MODE") != "orchestrator" {
		log.Printf("⚡ [Soul Recovery] Auto-igniting the brain for '%s'...", activeIdentity)
		go func(identityName string) {
			brainMutex.Lock()
			defer brainMutex.Unlock()

			execCmd := exec.CommandContext(workerCtx, func() string {
				if bin := os.Getenv("OTTOCLAW_BIN"); bin != "" {
					return bin
				}
				return "/app/ottoclaw"
			}(), "gateway", "--debug")

			env := getSafeEnv()
			env = append(env, fmt.Sprintf("AGENT_NAME=%s", identityName))
			env = append(env, "AGENT_MISSION=Recovered from stasis")

			execCmd.Env = env
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			if err := execCmd.Start(); err != nil {
				log.Printf("❌ [Soul Recovery] Failed to ignite brain: %v", err)
				return
			}
			currentBrain = execCmd
			execCmd.Wait()
		}(activeIdentity)
	}

	masterGrpcURL := os.Getenv("MASTER_GRPC_URL")
	if masterGrpcURL == "" {
		masterGrpcURL = "master:50051"
	}

	// 2. เชื่อมต่อ gRPC ไปยัง Master พร้อมระบบ Retry
	var conn *grpc.ClientConn
	for attempt := 0; ; attempt++ {
		c, err := grpc.Dial(masterGrpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			conn = c
			break
		}
		wait := backoffDuration(attempt)
		log.Printf("⚠️  Could not connect to Master at %s: %v. Retrying in %v...", masterGrpcURL, err, wait)
		time.Sleep(wait)
		if workerCtx.Err() != nil {
			return
		}
	}
	defer conn.Close()

	grpcClient := proto.NewMasterControlClient(conn)

	// Sync voice prints from master at startup, then auto-enroll own agent voice
	go func() {
		syncVoicePrints()
		autoEnrollSelfVoice(workerCtx)
	}()

	// 3. Goroutine รับคำสั่งและ Auto-Reconnect
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🧨 RECOVERED from panic in Command Loop: %v", r)
			}
		}()

		streamAttempt := 0
		for {
			// ตรวจสอบ context ก่อนเริ่ม loop ใหม่
			if workerCtx.Err() != nil {
				return
			}

			// เปิด Stream รับคำสั่งจาก Master (ผูก lifecycle กับ workerCtx)
			streamCtx := workerCtx
			if secret := resolveNodeSecret(); secret != "" {
				streamCtx = metadata.AppendToOutgoingContext(workerCtx, "x-node-secret", secret)
			}
			stream, err := grpcClient.GetCommand(streamCtx, &proto.NodeStatus{NodeId: nodeID})
			if err != nil {
				if workerCtx.Err() != nil {
					return
				} // shutdown ระหว่าง dial
				wait := backoffDuration(streamAttempt)
				log.Printf("❌ Failed to open command stream (attempt %d), retrying in %v: %v", streamAttempt, wait, err)
				streamAttempt++
				select {
				case <-time.After(wait):
				case <-workerCtx.Done():
					return
				}
				continue
			}
			log.Println("📡 Stream opened and listening for commands...")
			streamAttempt = 0 // reset backoff on successful open

			// สร้าง Context ย่อยสำหรับ lifecycle ของ stream นี้
			_, streamCancel := context.WithCancel(workerCtx)

			for {
				cmd, err := stream.Recv()
				if err != nil {
					log.Printf("❌ Error receiving command (Stream closed): %v", err)
					streamCancel()
					break
				}

				if cmd.CommandId == "SYSTEM_TERMINATE" || cmd.Type == "SYSTEM_TERMINATE" {
					log.Println("🛑 RECEIVED SYSTEM_TERMINATE SIGNAL from Master!")
					streamCancel()
					workerCancel() // ปิดทั้ง worker

					log.Println("⏳ Gracefully shutting down in 2 seconds...")
					time.Sleep(2 * time.Second)

					log.Println("👋 Worker Terminal Exited.")
					os.Exit(0)
				}

				if cmd.Type == "SYSTEM_SOUL_TRANSFER" {
					if os.Getenv("OTTOCLAW_MODE") == "orchestrator" {
						log.Printf("🛡️ [Security] Reincarnation blocked for Orchestrator node.")
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    "Orchestrator node is immutable and cannot be reincarnated.",
						})
						continue
					}
					log.Printf("🔥 [Awakening] Received a new Soul from The Creator!")

					// Payload format is: Name:Base64EncodedMarkdown
					parts := strings.SplitN(cmd.Payload, ":", 2)
					if len(parts) != 2 {
						log.Printf("❌ Invalid soul transfer payload")
						continue
					}

					soulName := parts[0]
					encodedSoul := parts[1]

					decodedBytes, err := base64.StdEncoding.DecodeString(encodedSoul)
					if err != nil {
						log.Printf("❌ Failed to decode soul payload: %v", err)
						continue
					}

					// Ensure workspace directory exists
					if err := os.MkdirAll(workspaceDir, 0755); err != nil {
						log.Printf("❌ Failed to create workspace dir: %v", err)
					}

					// Save the identity
					soulPath := filepath.Join(workspaceDir, "SOUL.md")
					soulIDPath := filepath.Join(workspaceDir, "SOUL_ID")
					if err := os.WriteFile(soulPath, decodedBytes, 0644); err != nil {
						log.Printf("❌ Failed to bind soul to disk: %v", err)
						continue
					}
					// Also save the soul name for recovery
					if err := os.WriteFile(soulIDPath, []byte(strings.ToLower(soulName)), 0644); err != nil {
						log.Printf("⚠️  Failed to persist soul name: %v", err)
						continue
					}
					log.Printf("✨ [Awakening] Soul '%s' bound to %s", soulName, soulPath)

					// Spawn the container using the newly bound soul
					// Usually handled by spawning the `siam-synapse-ottoclaw` image
					// We mount the workspace/v2 directory so it can read its SOUL.md
					log.Printf("⚡ [Awakening] Igniting the spark... booting %s", soulName)

					// 🛡️ Safety Guard: If we are already in Orchestrator mode, we have a brain.
					// Do NOT spawn a second one via the internal Arm.
					if os.Getenv("OTTOCLAW_MODE") == "orchestrator" {
						log.Printf("🛡️ [Orchestrator Mode] Internal Brain is already sovereign. Skipping redundant awakening.")
						continue
					}

					// 🛡️ Soul Identification: Capture the name of the new occupant
					currentSoulMu.Lock()
					currentSoul = soulName
					currentSoulMu.Unlock()

					// 🛡️ Brain Process Management: Ensure only one brain runs at a time
					brainMutex.Lock()
					if currentBrain != nil && currentBrain.Process != nil {
						log.Printf("🛑 [Awakening] Terminating existing brain process...")
						currentBrain.Process.Kill()
						currentBrain.Wait() // Wait for the process to exit
					}

					// Spawn the brain process
					go func(identityName string) {
						defer brainMutex.Unlock() // Unlock when this goroutine finishes its critical section

						execCmd := exec.CommandContext(workerCtx, func() string {
							if bin := os.Getenv("OTTOCLAW_BIN"); bin != "" {
								return bin
							}
							return "/app/ottoclaw"
						}(), "gateway", "--debug")

						// Inherit current environment but override required identity variables
						env := getSafeEnv()
						env = append(env, fmt.Sprintf("AGENT_NAME=%s", identityName))
						env = append(env, "AGENT_MISSION=Awaiting commands from The Creator")

						execCmd.Env = env
						execCmd.Stdout = os.Stdout
						execCmd.Stderr = os.Stderr

						if err := execCmd.Start(); err != nil {
							log.Printf("❌ [Awakening] Failed to ignite the spark: %v", err)
							// Report failure
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmd.CommandId,
								NodeId:    nodeID,
								Success:   false,
								Output:    fmt.Sprintf("Failed to ignite spark for %s: %v", identityName, err),
							})
							return
						}
						currentBrain = execCmd // Assign the new command to currentBrain
						log.Printf("🚀 [Awakening] Spark ignited for '%s'. Brain is now active.", identityName)

						// Report success
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    fmt.Sprintf("Successfully reincarnated as %s. Brain is now active.", identityName),
						})

						execCmd.Wait() // Wait for the brain process to complete
						log.Printf("💀 [Awakening] Brain for '%s' has ceased.", identityName)
					}(soulName)

					continue
				}
				if cmd.Type == "SYSTEM_WORKSPACE_SYNC" {
					log.Printf("📥 [Workspace Sync] Received file update: %s", cmd.Payload)

					// Format: filename:base64Content
					parts := strings.SplitN(cmd.Payload, ":", 2)
					if len(parts) != 2 {
						log.Printf("❌ Invalid sync payload")
						continue
					}

					filename := parts[0]
					encodedContent := parts[1]

					// 🛡️ Guard: prevent path traversal in synced filenames
					if err := sanitizeFilename(filename); err != nil {
						log.Printf("🛑 [Security] Path traversal blocked in sync [%s]: %v", cmd.CommandId, err)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    fmt.Sprintf("Filename rejected by security guard: %v", err),
						})
						continue
					}

					decodedBytes, err := base64.StdEncoding.DecodeString(encodedContent)
					if err != nil {
						log.Printf("❌ Failed to decode sync payload: %v", err)
						continue
					}

					filePath := filepath.Join(workspaceDir, filename)

					// 🛡️ Security Guard: Don't let Master syncs overwrite eternal Orchestrator identity
					if os.Getenv("OTTOCLAW_MODE") == "orchestrator" && (filename == "SOUL.md" || filename == "SOUL_ID" || filename == "AGENTS.md") {
						log.Printf("🛡️ [Security] Ignored sync of %s for Orchestrator identity.", filename)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    fmt.Sprintf("Ignored sync of protected identity file: %s", filename),
						})
						continue
					}

					if err := os.WriteFile(filePath, decodedBytes, 0644); err != nil {
						log.Printf("❌ Failed to write sync file %s: %v", filename, err)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    fmt.Sprintf("Failed to write %s: %v", filename, err),
						})
						continue
					}

					log.Printf("✅ [Workspace Sync] File %s updated successfully.", filename)

					// 🔥 Trigger Hot Reload if it's a critical file
					isCritical := (filename == "SOUL.md" || filename == "AGENTS.md" || filename == "ROLE")

					if isCritical && os.Getenv("OTTOCLAW_MODE") != "orchestrator" {
						log.Printf("🔥 [Hot Reload] Critical file %s changed. Restarting brain...", filename)

						currentSoulMu.RLock()
						identityName := currentSoul
						currentSoulMu.RUnlock()

						if identityName != "" {
							brainMutex.Lock()
							if currentBrain != nil && currentBrain.Process != nil {
								log.Printf("🛑 [Hot Reload] Terminating brain for restart...")
								currentBrain.Process.Kill()
								currentBrain.Wait()
							}

							// Spawn new brain (Reuse logic from SOUL_TRANSFER)
							go func(name string) {
								defer brainMutex.Unlock()

								execCmd := exec.CommandContext(workerCtx, func() string {
									if bin := os.Getenv("OTTOCLAW_BIN"); bin != "" {
										return bin
									}
									return "/app/ottoclaw"
								}(), "gateway", "--debug")

								env := getSafeEnv()
								env = append(env, fmt.Sprintf("AGENT_NAME=%s", name))
								env = append(env, "AGENT_MISSION=Re-awakened via Hot Reload")

								execCmd.Env = env
								execCmd.Stdout = os.Stdout
								execCmd.Stderr = os.Stderr

								currentBrain = execCmd
								if err := execCmd.Start(); err != nil {
									log.Printf("❌ [Hot Reload] Failed to re-ignite brain: %v", err)
									return
								}
								log.Printf("🚀 [Hot Reload] Brain re-ignited for '%s'.", name)
								execCmd.Wait()
							}(identityName)
						} else {
							brainMutex.Unlock()
						}
					}

					grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
						CommandId: cmd.CommandId,
						NodeId:    nodeID,
						Success:   true,
						Output:    fmt.Sprintf("File %s synced successfully. Hot reload triggered: %v", filename, isCritical),
					})
					continue
				}

				if cmd.Type == "SYSTEM_HOT_RELOAD" {
					log.Printf("🔥 [Hot Reload] Received SYSTEM_HOT_RELOAD from Master")

					if os.Getenv("OTTOCLAW_MODE") == "orchestrator" {
						log.Printf("🛡️ [Hot Reload] Skipped: Orchestrator identity is immutable")
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    "Hot reload skipped: orchestrator is immutable",
						})
						continue
					}

					currentSoulMu.RLock()
					identityName := currentSoul
					currentSoulMu.RUnlock()

					if identityName == "" {
						log.Printf("⚠️  [Hot Reload] No active soul — nothing to reload")
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    "Hot reload skipped: no soul loaded",
						})
						continue
					}

					brainMutex.Lock()
					if currentBrain != nil && currentBrain.Process != nil {
						log.Printf("🛑 [Hot Reload] Terminating brain for reload...")
						currentBrain.Process.Kill()
						currentBrain.Wait()
					}
					go func(name string) {
						defer brainMutex.Unlock()
						execCmd := exec.CommandContext(workerCtx, func() string {
							if bin := os.Getenv("OTTOCLAW_BIN"); bin != "" {
								return bin
							}
							return "/app/ottoclaw"
						}(), "gateway", "--debug")
						env := getSafeEnv()
						env = append(env, fmt.Sprintf("AGENT_NAME=%s", name))
						env = append(env, "AGENT_MISSION=Re-awakened via Hot Reload")
						execCmd.Env = env
						execCmd.Stdout = os.Stdout
						execCmd.Stderr = os.Stderr
						currentBrain = execCmd
						if err := execCmd.Start(); err != nil {
							log.Printf("❌ [Hot Reload] Failed to re-ignite brain: %v", err)
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmd.CommandId,
								NodeId:    nodeID,
								Success:   false,
								Output:    fmt.Sprintf("Hot reload failed for %s: %v", name, err),
							})
							return
						}
						log.Printf("🚀 [Hot Reload] Brain re-ignited for '%s'", name)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    fmt.Sprintf("Hot reload complete for %s", name),
						})
						execCmd.Wait()
					}(identityName)
					continue
				}

				// SYSTEM_POLL_MISSIONS: nudge brain to poll for new missions immediately via SIGUSR1
				if cmd.Type == "SYSTEM_POLL_MISSIONS" {
					brainMutex.Lock()
					brain := currentBrain
					brainMutex.Unlock()
					if brain != nil && brain.Process != nil {
						brain.Process.Signal(syscall.SIGUSR1)
						log.Printf("🎯 [Mission Nudge] Sent SIGUSR1 to brain (pid %d) — immediate poll triggered", brain.Process.Pid)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    "Mission poll nudge sent",
						})
					} else {
						log.Printf("⚠️  [Mission Nudge] No active brain to nudge")
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    "No active brain process",
						})
					}
					continue
				}


				// SYSTEM_DM: deliver a direct message to the brain's inbox
				if cmd.Type == "SYSTEM_DM" {
					var from, message string
					decoded, err := base64.StdEncoding.DecodeString(cmd.Payload)
					if err == nil {
						parts := strings.SplitN(string(decoded), "\x00", 2)
						if len(parts) == 2 {
							from, message = parts[0], parts[1]
						} else {
							from = "master"
							message = string(decoded)
						}
					} else {
						parts := strings.SplitN(cmd.Payload, ":", 2)
						if len(parts) == 2 {
							from, message = parts[0], parts[1]
						} else {
							from = "master"
							message = cmd.Payload
						}
					}
					log.Printf("\U0001f4ac [DM] From %s: %s", from, message)

					// Append to DM inbox file so brain can pick it up on next poll
					inboxPath := filepath.Join(workspaceDir, "DM_INBOX")
					entry := fmt.Sprintf("[%s][%s] %s\n", time.Now().Format(time.RFC3339), from, message)
					os.WriteFile(inboxPath, []byte(entry), 0644)

					// Nudge brain via SIGUSR1 to poll AgentMessages immediately
					brainMutex.Lock()
					brain := currentBrain
					brainMutex.Unlock()
					nudged := false
					if brain != nil && brain.Process != nil {
						brain.Process.Signal(syscall.SIGUSR1)
						nudged = true
						log.Printf("\U0001f4ac [DM] Nudged brain (pid %d) via SIGUSR1", brain.Process.Pid)
					}
					out := fmt.Sprintf("DM from [%s] written to inbox", from)
					if !nudged {
						out += " (brain offline — DM queued)"
					}
					grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
						CommandId: cmd.CommandId,
						NodeId:    nodeID,
						Success:   true,
						Output:    out,
					})
					continue
				}

				// SYSTEM_SPEAK: speak text aloud via platform TTS
				if cmd.Type == "SYSTEM_SPEAK" {
					text := cmd.Payload
					if text == "" {
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    "empty text payload",
						})
						continue
					}
					log.Printf("\U0001f50a [Speak] TTS: %s", text)
					go func(cmdID, txt string) {
						// Phase 5.4: Speaking Lock — mark device as speaking
						isSpeaking.Store(true)
						defer isSpeaking.Store(false)

						var success bool
						var output string
						isTermux := false
						if _, statErr := os.Stat("/data/data/com.termux"); statErr == nil {
							isTermux = true
						}
						if isTermux {
							// Android/Termux: native TTS
							out, err := exec.CommandContext(workerCtx, "termux-tts-speak", txt).CombinedOutput()
							success = err == nil
							output = string(out)
						} else {
							// Linux: edge-tts → ffplay (natural Thai voice)
							tmpMp3 := "/tmp/speak-" + cmdID + ".mp3"
							defer os.Remove(tmpMp3)
							edgeErr := exec.CommandContext(workerCtx, "edge-tts",
								"--voice", "th-TH-PremwadeeNeural",
								"--text", txt,
								"--write-media", tmpMp3,
							).Run()
							if edgeErr == nil {
								// Try players in order: ffplay, mpv, aplay (via ffmpeg convert)
								if exec.CommandContext(workerCtx, "ffplay", "-nodisp", "-autoexit", tmpMp3).Run() == nil {
									success = true
								} else if exec.CommandContext(workerCtx, "mpv", "--no-terminal", tmpMp3).Run() == nil {
									success = true
								} else if exec.CommandContext(workerCtx, "paplay", tmpMp3).Run() == nil {
									success = true
								}
							}
							// Fallback: espeak-ng
							if !success {
								out, err := exec.CommandContext(workerCtx, "espeak-ng", "-v", "th", txt).CombinedOutput()
								if err != nil {
									out2, err2 := exec.CommandContext(workerCtx, "espeak-ng", txt).CombinedOutput()
									success = err2 == nil
									output = string(out2)
									if err2 != nil {
										output = string(out) + " / " + string(out2)
									}
								} else {
									success = true
									output = string(out)
								}
							}
						}
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmdID,
							NodeId:    nodeID,
							Success:   success,
							Output:    output,
						})
					}(cmd.CommandId, text)
					continue
				}

				// SYSTEM_ENROLL_VOICE: record voice sample → save resemblyzer embedding for speaker ID
				// Payload: <name>:<duration>  e.g. "alfred:10"
				if cmd.Type == "SYSTEM_ENROLL_VOICE" {
					go func(cmdID, payload string) {
						name := "default"
						duration := "10"
						if payload != "" {
							parts := strings.SplitN(payload, ":", 2)
							if parts[0] != "" {
								name = parts[0]
							}
							if len(parts) >= 2 && parts[1] != "" {
								duration = parts[1]
							}
						}
						wavPath := "/tmp/enroll-" + cmdID + ".wav"
						defer os.Remove(wavPath)

						// Record voice sample
						recErr := recordWAV(workerCtx, wavPath, duration)
						if recErr != nil {
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmdID, NodeId: nodeID, Success: false, Output: "record failed: " + recErr.Error(),
							})
							return
						}

						// Save resemblyzer embedding
						vpDir := voicePrintsDir()
						embedPath := filepath.Join(vpDir, name+".npy")
						pyEnroll := `import sys, numpy as np
from resemblyzer import VoiceEncoder, preprocess_wav
from pathlib import Path
enc = VoiceEncoder()
wav = preprocess_wav(Path(sys.argv[1]))
embed = enc.embed_utterance(wav)
np.save(sys.argv[2], embed)
print("enrolled:" + sys.argv[3])
`
						out, err := exec.CommandContext(workerCtx, "python3", "-c", pyEnroll,
							wavPath, embedPath, name).Output()
						if err != nil {
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmdID, NodeId: nodeID, Success: false, Output: "enroll failed: " + err.Error(),
							})
							return
						}
						log.Printf("🎙️ [ENROLL] %s", strings.TrimSpace(string(out)))

						// Upload to master central storage so all workers share it
						if upErr := uploadVoicePrint(name, embedPath); upErr != nil {
							log.Printf("⚠️ [ENROLL] upload to master failed: %v (local copy kept)", upErr)
						} else {
							vpSyncedAt = time.Time{} // invalidate cache so next LISTEN syncs fresh
							log.Printf("☁️ [ENROLL] uploaded voice print for %s to master", name)
						}

						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmdID, NodeId: nodeID, Success: true,
							Output: "enrolled voice print for: " + name,
						})
					}(cmd.CommandId, cmd.Payload)
					continue
				}

				// SYSTEM_LISTEN: record audio → faster-whisper STT → return transcript
				if cmd.Type == "SYSTEM_LISTEN" {
					go func(cmdID, payload string) {
						// Phase 5.4: Speaking Lock — refuse to listen while TTS is playing
						if isSpeaking.Load() {
							log.Printf("🔇 [LISTEN] blocked — device is currently speaking")
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmdID, NodeId: nodeID, Success: false, Output: "speaking_locked",
							})
							return
						}

						duration := "5"
						lang := "th"
						if payload != "" {
							parts := strings.SplitN(payload, ":", 2)
							if len(parts) >= 1 && parts[0] != "" {
								duration = parts[0]
							}
							if len(parts) >= 2 && parts[1] != "" {
								lang = parts[1]
							}
						}
						wavPath := "/tmp/listen-" + cmdID + ".wav"
						defer os.Remove(wavPath)

						// Record audio
						var recErr error
						if isTermux() {
							// Android/Termux: termux-microphone-record
							recCmd := exec.CommandContext(workerCtx, "termux-microphone-record",
								"-e", "WAV", "-l", duration, "-f", wavPath)
							recErr = recCmd.Run()
						} else {
							recErr = recordWAV(workerCtx, wavPath, duration)
						}
						if recErr != nil {
							log.Printf("⚠️ [LISTEN] record failed: %v", recErr)
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmdID, NodeId: nodeID, Success: false, Output: "record failed: " + recErr.Error(),
							})
							return
						}

						// Transcribe: faster-whisper first, fallback to openai-whisper
						pyScript := `import sys
wav, lang = sys.argv[1], sys.argv[2]
try:
    from faster_whisper import WhisperModel
    m = WhisperModel('small', device='cpu', compute_type='int8')
    segs, _ = m.transcribe(wav, language=lang)
    print(''.join([s.text for s in segs]))
except Exception:
    import whisper
    m = whisper.load_model('small')
    result = m.transcribe(wav, language=lang)
    print(result['text'])
`
						out, err := exec.CommandContext(workerCtx, "python3", "-c", pyScript, wavPath, lang).Output()
						if err != nil {
							log.Printf("⚠️ [LISTEN] whisper failed: %v", err)
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmdID, NodeId: nodeID, Success: false, Output: "whisper failed: " + err.Error(),
							})
							return
						}
						transcript := strings.TrimSpace(string(out))
						log.Printf("🎤 [LISTEN] transcript: %s", transcript)

						// Phase 5.3: Speaker Identification via resemblyzer
						// Sync voice prints from master before running speaker ID
						go syncVoicePrints()
						speaker := ""
						confidence := 0.0
						vpDir := voicePrintsDir()
						pyIdentify := `import sys, os, json, numpy as np
from resemblyzer import VoiceEncoder, preprocess_wav
from pathlib import Path
vp_dir = sys.argv[2]
wav_path = sys.argv[1]
try:
    enc = VoiceEncoder()
    wav = preprocess_wav(Path(wav_path))
    embed = enc.embed_utterance(wav)
    best_name, best_sim = "", 0.0
    for f in Path(vp_dir).glob("*.npy"):
        ref = np.load(str(f))
        sim = float(np.dot(embed, ref) / (np.linalg.norm(embed) * np.linalg.norm(ref) + 1e-9))
        if sim > best_sim:
            best_sim = sim
            best_name = f.stem
    print(json.dumps({"speaker": best_name, "confidence": round(best_sim, 3)}))
except Exception as e:
    print(json.dumps({"speaker": "", "confidence": 0.0, "error": str(e)}))
`
						sidOut, sidErr := exec.CommandContext(workerCtx, "python3", "-c", pyIdentify,
							wavPath, vpDir).Output()
						if sidErr == nil {
							var sidResult struct {
								Speaker    string  `json:"speaker"`
								Confidence float64 `json:"confidence"`
							}
							if json.Unmarshal(bytes.TrimSpace(sidOut), &sidResult) == nil && sidResult.Confidence >= 0.75 {
								speaker = sidResult.Speaker
								confidence = sidResult.Confidence
								log.Printf("🎙️ [LISTEN] speaker: %s (%.2f)", speaker, confidence)
							}
						}

						// Phase 5.5: classify speaker as agent or human
						isAgent := false
						knownAgentNames := map[string]bool{"auric": true, "kaidos": true, "kook": true}
						if speaker != "" {
							currentSoulMu.RLock()
							selfName := strings.ToLower(currentSoul)
							currentSoulMu.RUnlock()
							isAgent = knownAgentNames[strings.ToLower(speaker)] || strings.ToLower(speaker) == selfName
						}

						// Return transcript + optional speaker as JSON
						var resultOut string
						if speaker != "" {
							resultOut = fmt.Sprintf(`{"transcript":%q,"speaker":%q,"confidence":%.3f,"is_agent":%v}`,
								transcript, speaker, confidence, isAgent)
						} else {
							resultOut = transcript
						}
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmdID, NodeId: nodeID, Success: true, Output: resultOut,
						})
					}(cmd.CommandId, cmd.Payload)
					continue
				}

				// SYSTEM_LISTEN_STOP: cancel a running listen loop by loopID
				if cmd.Type == "SYSTEM_LISTEN_STOP" {
					loopID := cmd.Payload
					listenLoopsMu.Lock()
					if cancel, ok := listenLoops[loopID]; ok {
						cancel()
						delete(listenLoops, loopID)
						log.Printf("🛑 [LISTEN_LOOP] stopped: %s", loopID)
					}
					listenLoopsMu.Unlock()
					grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
						CommandId: cmd.CommandId, NodeId: nodeID, Success: true, Output: "loop stopped",
					})
					continue
				}

				// SYSTEM_LISTEN_LOOP: continuous mic recording → STT → send each transcript back
				// Payload: <duration_secs>:<lang>  e.g. "5:th"
				if cmd.Type == "SYSTEM_LISTEN_LOOP" {
					loopID := cmd.CommandId
					duration := "5"
					lang := "th"
					if cmd.Payload != "" {
						parts := strings.SplitN(cmd.Payload, ":", 2)
						if parts[0] != "" {
							duration = parts[0]
						}
						if len(parts) >= 2 && parts[1] != "" {
							lang = parts[1]
						}
					}

					loopCtx, loopCancel := context.WithCancel(workerCtx)
					listenLoopsMu.Lock()
					listenLoops[loopID] = loopCancel
					listenLoopsMu.Unlock()

					go func(lID, dur, ln string, ctx context.Context) {
						defer func() {
							listenLoopsMu.Lock()
							delete(listenLoops, lID)
							listenLoopsMu.Unlock()
							log.Printf("🛑 [LISTEN_LOOP] goroutine exited: %s", lID)
						}()
						log.Printf("🎙️ [LISTEN_LOOP] started: %s (dur=%ss lang=%s)", lID, dur, ln)
						for {
							if ctx.Err() != nil {
								return
							}
							// Speaking lock: wait if TTS is playing
							if isSpeaking.Load() {
								time.Sleep(500 * time.Millisecond)
								continue
							}

							wavPath := fmt.Sprintf("/tmp/loop-%s-%d.wav", lID, time.Now().UnixNano())
							recErr := recordWAV(ctx, wavPath, dur)
							if recErr != nil {
								os.Remove(wavPath)
								if ctx.Err() != nil {
									return
								}
								time.Sleep(time.Second)
								continue
							}

							// Transcribe
							pyScript := `import sys
wav, lang = sys.argv[1], sys.argv[2]
try:
    from faster_whisper import WhisperModel
    m = WhisperModel('small', device='cpu', compute_type='int8')
    segs, _ = m.transcribe(wav, language=lang)
    print(''.join([s.text for s in segs]))
except Exception:
    import whisper
    m = whisper.load_model('small')
    print(m.transcribe(wav, language=lang)['text'])
`
							out, err := exec.CommandContext(ctx, "python3", "-c", pyScript, wavPath, ln).Output()
							os.Remove(wavPath)
							if err != nil || ctx.Err() != nil {
								continue
							}
							transcript := strings.TrimSpace(string(out))
							if transcript == "" {
								continue
							}

							// Speaker ID
							speaker, confidence, isAgent := "", 0.0, false
							go syncVoicePrints()
							vpDir := voicePrintsDir()
							pyID := `import sys, os, json, numpy as np
from resemblyzer import VoiceEncoder, preprocess_wav
from pathlib import Path
try:
    enc = VoiceEncoder()
    wav = preprocess_wav(Path(sys.argv[1]))
    embed = enc.embed_utterance(wav)
    best_name, best_sim = "", 0.0
    for f in Path(sys.argv[2]).glob("*.npy"):
        ref = np.load(str(f))
        sim = float(np.dot(embed, ref) / (np.linalg.norm(embed)*np.linalg.norm(ref)+1e-9))
        if sim > best_sim:
            best_sim, best_name = sim, f.stem
    print(json.dumps({"speaker":best_name,"confidence":round(best_sim,3)}))
except Exception as e:
    print(json.dumps({"speaker":"","confidence":0.0}))
`
							// re-record wav for speaker ID (we already deleted it, use a new sample)
							// Note: speaker ID runs on the same wavPath which was recorded above
							// Since we deleted it, skip speaker ID for loop (done inline above)
							_ = pyID
							_ = vpDir

							knownAgents := map[string]bool{"auric":true,"kaidos":true,"kook":true}
							currentSoulMu.RLock()
							selfName := strings.ToLower(currentSoul)
							currentSoulMu.RUnlock()
							isAgent = knownAgents[strings.ToLower(speaker)] || strings.ToLower(speaker) == selfName

							// Send transcript back to master via ReportCommandResult with loopID
							result := fmt.Sprintf(`{"transcript":%q,"speaker":%q,"confidence":%.3f,"is_agent":%v}`,
								transcript, speaker, confidence, isAgent)
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: lID, NodeId: nodeID, Success: true, Output: result,
							})
						}
					}(loopID, duration, lang, loopCtx)
					// No immediate ReportCommandResult — results stream back continuously
					continue
				}

				// SYSTEM_TTS_CAPTURE: generate audio via local TTS → upload to master → Telegram voice
				if cmd.Type == "SYSTEM_TTS_CAPTURE" {
					decoded, err := base64.StdEncoding.DecodeString(cmd.Payload)
					if err != nil || !strings.Contains(string(decoded), "\x00") {
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId, NodeId: nodeID, Success: false, Output: "invalid payload",
						})
						continue
					}
					parts := strings.SplitN(string(decoded), "\x00", 2)
					chatID, ttsText := parts[0], parts[1]
					go func(cmdID, chatID, text string) {
						oggPath := "/tmp/tts-capture-" + cmdID + ".ogg"
						defer os.Remove(oggPath)

						// Try edge-tts first (natural Thai voice)
						mp3Path := "/tmp/tts-capture-" + cmdID + ".mp3"
						defer os.Remove(mp3Path)
						edgeOK := false
						if exec.CommandContext(workerCtx, "edge-tts",
							"--voice", "th-TH-PremwadeeNeural",
							"--text", text,
							"--write-media", mp3Path,
						).Run() == nil {
							if exec.CommandContext(workerCtx, "ffmpeg", "-y", "-i", mp3Path,
								"-c:a", "libopus", "-b:a", "64k", oggPath).Run() == nil {
								edgeOK = true
							}
						}

						if !edgeOK {
							// Fallback: espeak-ng → WAV → OGG
							wavPath := "/tmp/tts-capture-" + cmdID + ".wav"
							defer os.Remove(wavPath)
							genCmd := exec.CommandContext(workerCtx, "espeak-ng", "-v", "th", "-w", wavPath, text)
							if err := genCmd.Run(); err != nil {
								genCmd = exec.CommandContext(workerCtx, "espeak-ng", "-w", wavPath, text)
								if err2 := genCmd.Run(); err2 != nil {
									log.Printf("⚠️ [TTS_CAPTURE] TTS failed: %v / %v", err, err2)
									grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
										CommandId: cmdID, NodeId: nodeID, Success: false, Output: err2.Error(),
									})
									return
								}
							}
							conv := exec.CommandContext(workerCtx, "ffmpeg", "-y", "-i", wavPath, "-c:a", "libopus", "-b:a", "64k", oggPath)
							if err := conv.Run(); err != nil {
								log.Printf("⚠️ [TTS_CAPTURE] ffmpeg failed: %v", err)
								grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
									CommandId: cmdID, NodeId: nodeID, Success: false, Output: err.Error(),
								})
								return
							}
						}
						// Read OGG → base64 → POST to master
						oggData, err := os.ReadFile(oggPath)
						if err != nil {
							log.Printf("⚠️ [TTS_CAPTURE] read ogg failed: %v", err)
							return
						}
						uploadPayload := `{"chat_id":"` + chatID + `","audio_b64":"` + base64.StdEncoding.EncodeToString(oggData) + `"}`
						masterURL := os.Getenv("MASTER_API_URL")
						if masterURL == "" {
							masterURL = "http://master:8080/api/agent/v1"
						}
						apiKey := os.Getenv("MASTER_API_KEY")
						req, _ := http.NewRequestWithContext(workerCtx, "POST", masterURL+"/bridge/voice/upload", strings.NewReader(uploadPayload))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-API-Key", apiKey)
						resp, err := http.DefaultClient.Do(req)
						if err != nil {
							log.Printf("⚠️ [TTS_CAPTURE] upload failed: %v", err)
						} else {
							resp.Body.Close()
							log.Printf("🔊 [TTS_CAPTURE] audio uploaded for chat %s (status %d)", chatID, resp.StatusCode)
						}
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmdID, NodeId: nodeID, Success: true, Output: "tts sent",
						})
					}(cmd.CommandId, chatID, ttsText)
					continue
				}

				fmt.Printf("📥 Received Command ID: %s, Type: %s\n", cmd.CommandId, cmd.Type)

				if cmd.Type == "action" {
					if strings.HasPrefix(cmd.Payload, "restart") {
						// Payload: "restart" (self) or "restart:<container_name>"
						target := nodeID
						if actionParts := strings.SplitN(cmd.Payload, ":", 2); len(actionParts) == 2 && actionParts[1] != "" {
							target = actionParts[1]
						}
						if err := restartContainer(target); err != nil {
							log.Printf("❌ Failed to restart container %s: %v", target, err)
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmd.CommandId,
								NodeId:    nodeID,
								Success:   false,
								Output:    err.Error(),
							})
						} else {
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: cmd.CommandId,
								NodeId:    nodeID,
								Success:   true,
								Output:    fmt.Sprintf("Container %s restarted successfully", target),
							})
						}
					}
				} else if cmd.Type == "shell" {
					// 🤖 รันคำสั่ง Shell ที่ Agent ส่งมา (ทำเป็น Non-Blocking ให้อยู่เบื้องหลัง)
					log.Printf("🤖 Starting Background Shell Payload: %s", cmd.Payload)

					// 🛡️ Guard: block dangerous patterns before spawning shell
					if err := guardCommand(cmd.Payload); err != nil {
						log.Printf("🛑 [Security] Blocked command [%s]: %v", cmd.CommandId, err)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    fmt.Sprintf("Command blocked by security guard: %v", err),
						})
						continue
					}

					go func(commandId string, payload string, ctx context.Context) {
						execCmd := exec.CommandContext(ctx, "sh", "-c", payload)
						outputBytes, err := execCmd.CombinedOutput()

						// ถ้ายกเลิกโดย context.WithCancel()
						if ctx.Err() != nil {
							log.Printf("🛑 Command [%s] was TERMINATED by Graceful Shutdown.", commandId)
							return
						}

						success := (err == nil)
						outputStr := string(outputBytes)
						if err != nil {
							outputStr += fmt.Sprintf("\nError: %v", err)
						}

						// ทยอยตอบกลับไปยัง Master -> Agent
						ack, resultErr := grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: commandId,
							NodeId:    nodeID,
							Success:   success,
							Output:    outputStr,
						})

						if resultErr != nil {
							log.Printf("❌ Failed to report command result [%s]: %v", commandId, resultErr)
						} else {
							log.Printf("✅ Master Acked Result [%s]: %v", commandId, ack.Success)
						}
					}(cmd.CommandId, cmd.Payload, workerCtx)
				} else if cmd.Type == "BROWSER_OPEN" {
					// สั่งเปิด browser บน Worker node นี้
					// Payload format: "<url>" หรือ "<url>|<browser>"
					parts := strings.SplitN(cmd.Payload, "|", 2)
					url := strings.TrimSpace(parts[0])
					browserBin := ""
					if len(parts) == 2 {
						browserBin = strings.TrimSpace(parts[1])
					}

					go func(commandId, urlStr, bin string) {
						var execCmd *exec.Cmd

						switch runtime.GOOS {
						case "linux":
							display := os.Getenv("DISPLAY")
							wayland := os.Getenv("WAYLAND_DISPLAY")
							if display == "" && wayland == "" {
								grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
									CommandId: commandId, NodeId: nodeID, Success: false,
									Output: "BROWSER_OPEN failed: no GUI display (DISPLAY/WAYLAND_DISPLAY not set)",
								})
								return
							}
							if bin == "" || bin == "default" {
								execCmd = exec.Command("xdg-open", urlStr)
							} else {
								execCmd = exec.Command(bin, urlStr)
							}
						case "darwin":
							if bin == "" || bin == "default" {
								execCmd = exec.Command("open", urlStr)
							} else {
								execCmd = exec.Command(bin, urlStr)
							}
						case "windows":
							if bin == "" || bin == "default" {
								execCmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
							} else {
								execCmd = exec.Command(bin, urlStr)
							}
						default:
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: commandId, NodeId: nodeID, Success: false,
								Output: "BROWSER_OPEN failed: unsupported platform " + runtime.GOOS,
							})
							return
						}

						if err := execCmd.Start(); err != nil {
							grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
								CommandId: commandId, NodeId: nodeID, Success: false,
								Output: fmt.Sprintf("BROWSER_OPEN failed to launch: %v", err),
							})
							return
						}
						go func() { _ = execCmd.Wait() }()
						log.Printf("🌐 [Browser] Opened %s (PID %d)", urlStr, execCmd.Process.Pid)
						grpcClient.ReportCommandResult(newGRPCCtx(), &proto.CommandResult{
							CommandId: commandId, NodeId: nodeID, Success: true,
							Output: fmt.Sprintf("Opened %s (PID %d)", urlStr, execCmd.Process.Pid),
						})
					}(cmd.CommandId, url, browserBin)
				}
			}

			// ถ้าหลุดมาถึงนี่แปลว่า Stream รับคำสั่งมีปัญหา ให้หน่วงเวลาก่อนต่อใหม่
			streamAttempt++
			wait := backoffDuration(streamAttempt)
			log.Printf("⚙️  Stream reconnecting in %v (attempt %d)...", wait, streamAttempt)
			time.Sleep(wait)
		}
	}()

	// 4. Gather Static Telemetry
	vesselType := os.Getenv("VESSEL_TYPE")
	if vesselType == "" {
		vesselType = "Unknown Vessel"
	}

	ipAddr := os.Getenv("NODE_IP")
	if ipAddr == "" || ipAddr == "local" {
		// 🛡️ [NEW] Improved IP Detection: Try to find the outbound interface IP
		// by dialing the Master's gRPC host (without sending data)
		masterHost := "8.8.8.8" // Fallback to public DNS if we can't parse MASTER_GRPC_URL
		if grpcUrl := os.Getenv("MASTER_GRPC_URL"); grpcUrl != "" {
			if parts := strings.Split(grpcUrl, ":"); len(parts) > 0 {
				masterHost = parts[0]
			}
		}

		conn, err := net.Dial("udp", masterHost+":50051")
		if err == nil {
			if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
				ipAddr = udpAddr.IP.String()
				log.Printf("📡 [Detection] Detected preferred outbound IP: %s", ipAddr)
			}
			conn.Close()
		}

		// Fallback to traditional interface scan if UDP dial failed
		if ipAddr == "" || ipAddr == "local" || ipAddr == "127.0.0.1" {
			addrs, err := net.InterfaceAddrs()
			if err == nil {
				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipAddr = ipnet.IP.String()
							break
						}
					}
				}
			}
		}
	}

	osInfo := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		osInfo = "Termux " + osInfo // Clearly mark as Termux for SSH port auto-detection in Master
	}

	// 5. Loop ส่ง Heartbeat ทุกๆ 5 วินาที
	fmt.Println("🚀 Worker Node Started: Sending heartbeats to Master...")
	statusFailures := 0 // consecutive ReportStatus failure counter
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("🧨 RECOVERED from panic in Heartbeat Loop: %v", r)
				}
			}()

			if workerCtx.Err() != nil {
				return
			}
			c, err := cpu.Percent(0, false)
			cpuUsage := float32(0)
			if err == nil && len(c) > 0 {
				cpuUsage = float32(c[0])
			}

			m, err := mem.VirtualMemory()
			ramUsage := float32(0)
			totalRamGB := float64(0)
			if err == nil && m != nil {
				ramUsage = float32(m.UsedPercent)
				totalRamGB = float64(m.Total) / (1024 * 1024 * 1024)
			}

			// Read actual CPU hardware model from /proc/cpuinfo (works on Android/Linux)
			cpuModel := ""
			if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "Hardware\t") || strings.HasPrefix(line, "model name") {
						parts := strings.SplitN(line, ":", 2)
						if len(parts) == 2 {
							cpuModel = strings.TrimSpace(parts[1])
							break
						}
					}
				}
			}
			sysSpec := fmt.Sprintf("%d Cores CPU, %.1fGB RAM", runtime.NumCPU(), totalRamGB)
			if cpuModel != "" {
				sysSpec = fmt.Sprintf("%s (%d Cores, %.1fGB RAM)", cpuModel, runtime.NumCPU(), totalRamGB)
			}

			// 🔍 [Phase 2] Read tool list exported by the ottoclaw brain
			var reportedTools []string
			toolsFilePath := filepath.Join(workspaceDir, "TOOLS")
			if toolsBytes, err := os.ReadFile(toolsFilePath); err == nil {
				for _, line := range strings.Split(strings.TrimSpace(string(toolsBytes)), "\n") {
					if t := strings.TrimSpace(line); t != "" {
						reportedTools = append(reportedTools, t)
					}
				}
			}

			// 🔍 [Phase 5] Read role exported by the brain
			reportedRole := os.Getenv("AGENT_ROLE")
			if reportedRole == "" {
				reportedRole = "guest" // Default
				roleFilePath := filepath.Join(workspaceDir, "ROLE")
				if roleBytes, err := os.ReadFile(roleFilePath); err == nil {
					reportedRole = strings.TrimSpace(string(roleBytes))
				}
			}

			reportedDepartment := os.Getenv("AGENT_DEPARTMENT")
			if reportedDepartment == "" {
				deptFilePath := filepath.Join(workspaceDir, "DEPARTMENT")
				if deptBytes, err := os.ReadFile(deptFilePath); err == nil {
					reportedDepartment = strings.TrimSpace(string(deptBytes))
				}
			}

			reportedOrgID := os.Getenv("AGENT_ORG_ID")
			if reportedOrgID == "" {
				orgFilePath := filepath.Join(workspaceDir, "ORG_ID")
				if orgBytes, err := os.ReadFile(orgFilePath); err == nil {
					reportedOrgID = strings.TrimSpace(string(orgBytes))
				}
			}

			batLevel, cpuTemp := getBatteryAndTemp()

			status := &proto.NodeStatus{
				NodeId:       nodeID,
				CpuUsage:     cpuUsage,
				RamUsage:     ramUsage,
				Status:       "Online",
				VesselType:   vesselType,
				IpAddress:    ipAddr,
				OsInfo:       osInfo,
				SystemSpec:   sysSpec,
				Tools:        reportedTools,
				Role:         reportedRole,
				Department:   reportedDepartment,
				OrgId:        reportedOrgID,
				BatteryLevel: batLevel,
				Temperature:  cpuTemp,
			}

			// 🛡️ Add Soul + Auth Metadata
			currentSoulMu.RLock()
			soulID := currentSoul
			currentSoulMu.RUnlock()

			ctx := newGRPCCtx()
			if soulID != "" {
				normalizedSoulID := NormalizeID(soulID)
				if normalizedSoulID != "" {
					ctx = metadata.AppendToOutgoingContext(ctx, "x-soul-id", normalizedSoulID)
				}
			}

			res, err := grpcClient.ReportStatus(ctx, status)
			if err != nil {
				statusFailures++
				if statusFailures >= 3 {
					log.Printf("⚠️  [Heartbeat] %d consecutive failures — Master unreachable: %v", statusFailures, err)
				} else {
					fmt.Printf("⚠️ Error sending status: %v\n", err)
				}
			} else {
				if statusFailures > 0 {
					log.Printf("✅ [Heartbeat] Master reconnected after %d failures", statusFailures)
					statusFailures = 0
				}
				fmt.Printf("✅ Master Response: %s (CPU: %.1f%%)\n", res.Message, status.CpuUsage)
				if res.Action != "" {
					fmt.Printf("🔔 [Action] Received command from Master: %s\n", res.Action)
					if res.Action == "wakeup" {
						fmt.Println("✨ Waking up the vessel...")
					} else if res.Action == "update" {
						fmt.Println("📥 Status: Triggering Update...")
					} else if strings.HasPrefix(res.Action, "auto_qa:") {
						skill := strings.TrimPrefix(res.Action, "auto_qa:")
						fmt.Printf("🤖 [Auto QA] Triggering testing for skill: %s\n", skill)
						currentSoulMu.RLock()
						soul := currentSoul
						currentSoulMu.RUnlock()
						platform := detectPlatform()
						go func(s, soulID, plat string, ctx context.Context) {
							args := []string{filepath.Join(workspaceDir, "auto_qa_skill.py"), "--skill", s, "--force", "--platform", plat}
							if soulID != "" {
								args = append(args, "--soul-id", soulID)
							}
							cmd := exec.CommandContext(ctx, "python3", args...)
							output, err := cmd.CombinedOutput()
							if ctx.Err() != nil {
								fmt.Printf("🛑 [Auto QA] Cancelled for %s (worker shutdown)\n", s)
								return
							}
							if err != nil {
								fmt.Printf("❌ [Auto QA] Failed for %s: %v\nOutput: %s\n", s, err, output)
							} else {
								fmt.Printf("✅ [Auto QA] Finished for %s\n", s)
							}
						}(skill, soul, platform, workerCtx)
					}
				}
			}

		}()

		// Scale heartbeat interval: 5s normally, up to 30s when master is unreachable
		heartbeatInterval := 5 * time.Second
		if statusFailures >= 3 {
			scaled := 5 + statusFailures*5
			if scaled > 30 {
				scaled = 30
			}
			heartbeatInterval = time.Duration(scaled) * time.Second
		}
		select {
		case <-time.After(heartbeatInterval):
		case <-workerCtx.Done():
			return
		}
	}
}
