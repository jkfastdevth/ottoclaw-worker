package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TermuxAPITool provides access to Android hardware features via the termux-api package.
type TermuxAPITool struct{}

func NewTermuxAPITool() *TermuxAPITool {
	return &TermuxAPITool{}
}

func (t *TermuxAPITool) Name() string {
	return "termux_api"
}

func (t *TermuxAPITool) Description() string {
	return "Access Android hardware features via termux-api. " +
		"Supported actions: vibrate, toast, battery-status, location, camera-info, camera-photo, video-start, video-stop, clipboard-get, clipboard-set. " +
		"Requires the termux-api package to be installed on the device."
}

func (t *TermuxAPITool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"vibrate",
					"toast",
					"battery-status",
					"location",
					"camera-info",
					"camera-photo",
					"video-start",
					"video-stop",
					"video-capture",
					"clipboard-get",
					"clipboard-set",
				},
				"description": "The termux-api action to execute.",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional arguments for the action. For vibrate: ['-d', '1000']. For toast: ['-s', 'Message']. For camera-photo: ['-c', '0', '/tmp/photo.jpg']. For video-capture: ['10', '1', '/tmp/vid.mp4'] (duration in sec, camera 0/1, output path).",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TermuxAPITool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return ErrorResult("action is required")
	}

	command := "termux-" + action

	// Check if termux-api is installed
	if _, err := exec.LookPath(command); err != nil {
		return ErrorResult(fmt.Sprintf("%s not found. Is termux-api installed? Run 'pkg install termux-api'", command))
	}

	cmdArgs := []string{}
	if rawArgs, ok := args["args"].([]any); ok {
		for _, arg := range rawArgs {
			cmdArgs = append(cmdArgs, fmt.Sprint(arg))
		}
	} else if strArgs, ok := args["args"].(string); ok {
		// Orchestrator map[string]string workaround
		// Pass space-separated or comma-separated if needed.
		// For simple args like "-c,0,/tmp/photo.jpg", split by comma
		if strings.Contains(strArgs, ",") {
			parts := strings.Split(strArgs, ",")
			for _, p := range parts {
				cmdArgs = append(cmdArgs, strings.TrimSpace(p))
			}
		} else {
			parts := strings.Split(strArgs, " ")
			for _, p := range parts {
				cmdArgs = append(cmdArgs, strings.TrimSpace(p))
			}
		}
	}

	if action == "video-capture" {
		// Custom virtual action for: video-capture <duration_sec> <camera_id> <output_path>
		// Since termux-api does not support video directly, we use IP Webcam + ffmpeg
		if len(cmdArgs) < 3 {
			return ErrorResult("video-capture requires 3 arguments: duration_sec, camera_id (0=back, 1=front), output_path")
		}
		durationSecStr := cmdArgs[0]
		camID := cmdArgs[1]
		outPath := cmdArgs[2]

		durationSec := 10
		fmt.Sscanf(durationSecStr, "%d", &durationSec)
		if durationSec <= 0 || durationSec > 300 {
			durationSec = 10
		}

		ipcam := os.Getenv("IP_WEBCAM_URL")
		if ipcam == "" {
			ipcam = "http://127.0.0.1:8080"
		}
		
		// If camID is specified, we can optionally switch IP webcam camera via API
		// curl -s "http://127.0.0.1:8080/settings/ffc?set=$(if [ "$camID" = "1" ]; then echo "on"; else echo "off"; fi)"
		ffc := "off"
		if camID == "1" {
			ffc = "on"
		}
		exec.Command("curl", "-s", fmt.Sprintf("%s/settings/ffc?set=%s", ipcam, ffc)).Run()
		
		videoURL := ipcam
		if !strings.HasSuffix(videoURL, ".mjpg") && !strings.HasSuffix(videoURL, "/video") {
			videoURL = strings.TrimRight(ipcam, "/") + "/video"
		}

		// Use ffmpeg to stream from the IP Webcam to MP4 for durationSec
		ffmpegCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-t", fmt.Sprintf("%d", durationSec), "-i", videoURL, "-c:v", "libx264", "-preset", "ultrafast", outPath)
		ffmpegCmd.Env = append(ffmpegCmd.Environ(), "LD_PRELOAD=")
		
		var ffmpegErr bytes.Buffer
		ffmpegCmd.Stderr = &ffmpegErr
		
		if err := ffmpegCmd.Run(); err != nil {
			return ErrorResult(fmt.Sprintf("Failed to capture video using IP Webcam & ffmpeg: %v\nstderr: %s", err, ffmpegErr.String()))
		}

		return &ToolResult{
			ForLLM:  fmt.Sprintf("Video captured successfully at %s", outPath),
			ForUser: fmt.Sprintf("Recorded %d seconds of video to %s", durationSec, outPath),
		}
	}

	cmd := exec.CommandContext(ctx, command, cmdArgs...)

	// Ensure we run in a clean environment to avoid issues
	cmd.Env = append(cmd.Environ(), "LD_PRELOAD=")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return ErrorResult(fmt.Sprintf("termux-api failed: %s", errMsg))
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = fmt.Sprintf("Action '%s' completed successfully.", action)
	}

	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}
