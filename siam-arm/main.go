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
// 	stream, err := grpcClient.GetCommand(context.Background(), &proto.NodeStatus{NodeId: nodeID})
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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"net"
	"runtime"

	"os/exec"

	"github.com/jkfastdevth/Siam-Synapse/proto" // เปลี่ยนเป็น path โปรเจคของคุณ
	"github.com/shirou/gopsutil/cpu"            // ต้องติดตั้งเพิ่ม: go get github.com/shirou/gopsutil/cpu
	"github.com/shirou/gopsutil/mem"            // ต้องติดตั้งเพิ่ม: go get github.com/shirou/gopsutil/mem
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"regexp"
)

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
	brainMutex      sync.Mutex
	currentBrain    *exec.Cmd
	currentSoul     string
	currentSoulMu   sync.RWMutex
)

// getSafeEnv returns a filtered environment slice for the brain process
func getSafeEnv() []string {
	env := os.Environ()
	safeEnv := make([]string, 0, len(env))
	
	isOrchestrator := (os.Getenv("OTTOCLAW_MODE") == "orchestrator")
	
	for _, e := range env {
		// 🛡️ Prevent Telegram Bot Conflict (409)
		// Only allow the token if we are explicitly the orchestrator
		if !isOrchestrator {
			if strings.HasPrefix(e, "WORKER_TELEGRAM_TOKEN=") || 
			   strings.HasPrefix(e, "TELEGRAM_BOT_TOKEN=") || 
			   strings.HasPrefix(e, "OTTOCLAW_CHANNELS_TELEGRAM_") ||
			   strings.HasPrefix(e, "TELEGRAM_ALLOW_FROM=") || 
			   strings.HasPrefix(e, "TELEGRAM_BRIDGE_CHAT_ID=") {
				log.Printf("🛡️ [Env] Stripping %s for non-orchestrator node", strings.Split(e, "=")[0])
				continue
			}
		}
		safeEnv = append(safeEnv, e)
	}

	// 🛡️ Explicitly disable Telegram for workers to prevent config.json fallback
	if !isOrchestrator {
		safeEnv = append(safeEnv, "OTTOCLAW_CHANNELS_TELEGRAM_ENABLED=false")
		safeEnv = append(safeEnv, "OTTOCLAW_CHANNELS_TELEGRAM_TOKEN=")
	}

	return safeEnv
}

// จำลองฟังก์ชัน restartContainer
func restartContainer(containerName string) error {
	log.Printf("🐳 Executing: docker restart %s", containerName)
	// ตรงนี้คือ logic สั่ง docker จริงๆ
	return nil
}

func main() {
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

	if activeIdentity != "" && os.Getenv("OTTOCLAW_MODE") != "orchestrator" {
		log.Printf("⚡ [Soul Recovery] Auto-igniting the brain for '%s'...", activeIdentity)
		go func(identityName string) {
			brainMutex.Lock()
			defer brainMutex.Unlock()
			
			execCmd := exec.Command(func() string {
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
			
			currentBrain = execCmd
			if err := execCmd.Start(); err != nil {
				log.Printf("❌ [Soul Recovery] Failed to ignite brain: %v", err)
				return
			}
			execCmd.Wait()
		}(activeIdentity)
	}

	// 2. สร้าง Context หลักของ Worker
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	masterGrpcURL := os.Getenv("MASTER_GRPC_URL")
	if masterGrpcURL == "" {
		masterGrpcURL = "master:50051"
	}

	// 2. เชื่อมต่อ gRPC ไปยัง Master พร้อมระบบ Retry
	var conn *grpc.ClientConn
	for {
		c, err := grpc.Dial(masterGrpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			conn = c
			break
		}
		log.Printf("⚠️  Could not connect to Master at %s: %v. Retrying in 5s...", masterGrpcURL, err)
		time.Sleep(5 * time.Second)
		if workerCtx.Err() != nil { return }
	}
	defer conn.Close()

	grpcClient := proto.NewMasterControlClient(conn)

	// 3. Goroutine รับคำสั่งและ Auto-Reconnect
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🧨 RECOVERED from panic in Command Loop: %v", r)
			}
		}()

		for {
			// ตรวจสอบ context ก่อนเริ่ม loop ใหม่
			if workerCtx.Err() != nil { return }

			// เปิด Stream รับคำสั่งจาก Master
			stream, err := grpcClient.GetCommand(context.Background(), &proto.NodeStatus{NodeId: nodeID})
			if err != nil {
				log.Printf("❌ Failed to get command stream, retrying in 5s: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			log.Println("📡 Stream opened and listening for commands...")

			// สร้าง Context ย่อยสำหรับ lifecycle ของ stream นี้
			streamCtx, streamCancel := context.WithCancel(workerCtx)

			for {
				cmd, err := stream.Recv()
				if err != nil {
					log.Printf("❌ Error receiving command (Stream closed): %v", err)
					streamCancel()
					break
				}

				if cmd.CommandId == "SYSTEM_TERMINATE" {
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
						grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
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
					if err := os.WriteFile(soulIDPath, []byte(soulName), 0644); err != nil {
						log.Printf("⚠️  Failed to persist soul name: %v", err)
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
						
						currentBrain = execCmd // Assign the new command to currentBrain
						if err := execCmd.Start(); err != nil {
							log.Printf("❌ [Awakening] Failed to ignite the spark: %v", err)
							// Report failure
							grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
								CommandId: cmd.CommandId,
								NodeId:    nodeID,
								Success:   false,
								Output:    fmt.Sprintf("Failed to ignite spark for %s: %v", identityName, err),
							})
							return
						}
						log.Printf("🚀 [Awakening] Spark ignited for '%s'. Brain is now active.", identityName)

						// Report success
						grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
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
					
					decodedBytes, err := base64.StdEncoding.DecodeString(encodedContent)
					if err != nil {
						log.Printf("❌ Failed to decode sync payload: %v", err)
						continue
					}
					
					workspaceDir := "/app/workspace/v2"
					filePath := filepath.Join(workspaceDir, filename)
					
					// 🛡️ Security Guard: Don't let Master syncs overwrite eternal Orchestrator identity
					if os.Getenv("OTTOCLAW_MODE") == "orchestrator" && (filename == "SOUL.md" || filename == "SOUL_ID" || filename == "AGENTS.md") {
						log.Printf("🛡️ [Security] Ignored sync of %s for Orchestrator identity.", filename)
						grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   true,
							Output:    fmt.Sprintf("Ignored sync of protected identity file: %s", filename),
						})
						continue
					}

					if err := os.WriteFile(filePath, decodedBytes, 0644); err != nil {
						log.Printf("❌ Failed to write sync file %s: %v", filename, err)
						grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
							CommandId: cmd.CommandId,
							NodeId:    nodeID,
							Success:   false,
							Output:    fmt.Sprintf("Failed to write %s: %v", filename, err),
						})
						continue
					}
					
					log.Printf("✅ [Workspace Sync] File %s updated successfully.", filename)
					
					// 🔥 Trigger Hot Reload if it's a critical file
					isCritical := (filename == "SOUL.md" || filename == "AGENTS.md")
					
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
					
					grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
						CommandId: cmd.CommandId,
						NodeId:    nodeID,
						Success:   true,
						Output:    fmt.Sprintf("File %s synced successfully. Hot reload triggered: %v", filename, isCritical),
					})
					continue
				}

				fmt.Printf("📥 Received Command ID: %s, Type: %s\n", cmd.CommandId, cmd.Type)

				if cmd.Type == "action" {
					if cmd.Payload == "restart" {
						err := restartContainer("sworker-ubuntu-01")
						if err != nil {
							log.Printf("❌ Failed to restart: %v", err)
						}
					}
				} else if cmd.Type == "shell" {
					// 🤖 รันคำสั่ง Shell ที่ Agent ส่งมา (ทำเป็น Non-Blocking ให้อยู่เบื้องหลัง)
					log.Printf("🤖 Starting Background Shell Payload: %s", cmd.Payload)

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
						ack, resultErr := grpcClient.ReportCommandResult(context.Background(), &proto.CommandResult{
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
					}(cmd.CommandId, cmd.Payload, streamCtx)
				}
			}

			// ถ้าหลุดมาถึงนี่แปลว่า Stream รับคำสั่งมีปัญหา ให้หน่วงเวลาก่อนต่อใหม่
			time.Sleep(5 * time.Second)
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
			localAddr := conn.LocalAddr().(*net.UDPAddr)
			ipAddr = localAddr.IP.String()
			conn.Close()
			log.Printf("📡 [Detection] Detected preferred outbound IP: %s", ipAddr)
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

	// 5. Loop ส่ง Heartbeat ทุกๆ 5 วินาที
	fmt.Println("🚀 Worker Node Started: Sending heartbeats to Master...")
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("🧨 RECOVERED from panic in Heartbeat Loop: %v", r)
				}
			}()

			if workerCtx.Err() != nil { return }
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

		sysSpec := fmt.Sprintf("%d Cores CPU, %.1fGB RAM", runtime.NumCPU(), totalRamGB)
		
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

		status := &proto.NodeStatus{
			NodeId:     nodeID,
			CpuUsage:   cpuUsage,
			RamUsage:   ramUsage,
			Status:     "Online",
			VesselType: vesselType,
			IpAddress:  ipAddr,
			OsInfo:     osInfo,
			SystemSpec: sysSpec,
			Tools:      reportedTools,
			Role:       reportedRole,
			Department: reportedDepartment,
			OrgId:      reportedOrgID,
		}

		// 🛡️ Add Soul Metadata Header (X-API implementation)
		currentSoulMu.RLock()
		soulID := currentSoul
		currentSoulMu.RUnlock()
		
		ctx := context.Background()
		if soulID != "" {
			normalizedSoulID := NormalizeID(soulID)
			if normalizedSoulID != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "x-soul-id", normalizedSoulID)
			}
		}

		res, err := grpcClient.ReportStatus(ctx, status)
		if err != nil {
			fmt.Printf("⚠️ Error sending status: %v\n", err)
		} else {
			fmt.Printf("✅ Master Response: %s (CPU: %.1f%%)\n", res.Message, status.CpuUsage)
		}

		}()

		time.Sleep(5 * time.Second)
	}
}
