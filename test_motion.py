import cv2
import time
import sys
import os

if len(sys.argv) < 3:
    print("Usage: python3 test_motion.py <ipcam_url> <timeout_sec>")
    print("Example: python3 test_motion.py http://127.0.0.1:8080/video 60")
    sys.exit(1)

url = sys.argv[1]
timeout = int(sys.argv[2])

print(f"Connecting to video stream: {url}")
cap = cv2.VideoCapture(url)
if not cap.isOpened():
    print("❌ Error: Could not open video stream.")
    sys.exit(1)

ret, frame1 = cap.read()
if not ret:
    print("❌ Error: Could not read first frame.")
    sys.exit(1)
ret, frame2 = cap.read()

print(f"✅ Connection successful! Waiting for motion (Timeout: {timeout}s)...")
start_time = time.time()
frame_count = 0

while cap.isOpened():
    elapsed = time.time() - start_time
    if elapsed > timeout:
        print("⏳ Timeout reached, no motion detected.")
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
        print(f"📸 Motion Detected! Saving frame to: {out_path}")
        print(f"RESULT_PATH={out_path}")
        sys.exit(0)

    frame1 = frame2
    ret, frame2 = cap.read()
    frame_count += 1
    
    # Print status every 30 frames
    if frame_count % 30 == 0:
        print(f"[Elapsed: {int(elapsed)}s] Still watching...")

cap.release()
sys.exit(0)
