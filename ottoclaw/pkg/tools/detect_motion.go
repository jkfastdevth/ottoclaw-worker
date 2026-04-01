package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type DetectMotionTool struct{}

func NewDetectMotionTool() *DetectMotionTool {
	return &DetectMotionTool{}
}

func (t *DetectMotionTool) Name() string {
	return "detect_motion"
}

func (t *DetectMotionTool) Description() string {
	return "Detect motion from an IP Webcam video stream and capture the frame when movement occurs. " +
		"Arguments: timeout_sec (integer, optional, default 60), timeout in seconds before giving up."
}

func (t *DetectMotionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds before giving up if no motion is detected. Default is 60.",
			},
		},
	}
}

func (t *DetectMotionTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	timeoutSec := 60
	if val, ok := args["timeout_sec"].(float64); ok {
		timeoutSec = int(val)
	} else if val, ok := args["timeout_sec"].(int); ok {
		timeoutSec = val
	}

	ipcam := os.Getenv("IP_WEBCAM_URL")
	if ipcam == "" {
		ipcam = "http://127.0.0.1:8080"
	}
	videoURL := ipcam
	if !strings.HasSuffix(videoURL, ".mjpg") && !strings.HasSuffix(videoURL, "/video") {
		videoURL = strings.TrimRight(ipcam, "/") + "/video"
	}

	pyScript := fmt.Sprintf(`import cv2
import time
import sys
import os

url = sys.argv[1]
timeout = int(sys.argv[2])

cap = cv2.VideoCapture(url)
if not cap.isOpened():
    print("Error: Could not open video stream.")
    sys.exit(1)

ret, frame1 = cap.read()
if not ret:
    print("Error: Could not read first frame.")
    sys.exit(1)
ret, frame2 = cap.read()

start_time = time.time()
while cap.isOpened():
    if time.time() - start_time > timeout:
        print("Timeout reached, no motion detected.")
        sys.exit(2)

    diff = cv2.absdiff(frame1, frame2)
    gray = cv2.cvtColor(diff, cv2.COLOR_BGR2GRAY)
    blur = cv2.GaussianBlur(gray, (5, 5), 0)
    _, thresh = cv2.threshold(blur, 20, 255, cv2.THRESH_BINARY)
    dilated = cv2.dilate(thresh, None, iterations=3)
    contours, _ = cv2.findContours(dilated, cv2.RETR_TREE, cv2.CHAIN_APPROX_SIMPLE)

    motion_detected = False
    for contour in contours:
        if cv2.contourArea(contour) < 900: # motion size threshold
            continue
        motion_detected = True
        break
        
    if motion_detected:
        out_path = f"/tmp/motion_detected_{int(time.time())}.jpg"
        cv2.imwrite(out_path, frame2)
        print(f"RESULT_PATH={out_path}")
        sys.exit(0)

    frame1 = frame2
    ret, frame2 = cap.read()

cap.release()
sys.exit(0)
`)

	pyTmpPath := "/tmp/detect_motion.py"
	if err := os.WriteFile(pyTmpPath, []byte(pyScript), 0644); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to write motion detection runner: %v", err))
	}

	// Wait an extra 5 seconds beyond python timeout for graceful termination
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec+5)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "python3", pyTmpPath, videoURL, fmt.Sprintf("%d", timeoutSec))
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		return ErrorResult(fmt.Sprintf("Motion detection failed/timeout: %s\n%s", err.Error(), outStr))
	}

	// Parse out RESULT_PATH
	path := ""
	for _, line := range strings.Split(outStr, "\n") {
		if strings.HasPrefix(line, "RESULT_PATH=") {
			path = strings.TrimPrefix(line, "RESULT_PATH=")
			break
		}
	}

	if path == "" {
		return ErrorResult(fmt.Sprintf("Motion detection completed but no RESULT_PATH found. Output:\n%s", outStr))
	}

	return &ToolResult{
		ForLLM:  fmt.Sprintf(`{"path":"%s","success":true}`, path),
		ForUser: fmt.Sprintf("Motion detected! Snapshot saved to: %s", path),
	}
}
