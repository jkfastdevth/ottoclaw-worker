package main

import (
	"context"
	"fmt"
	"github.com/sipeed/ottoclaw/pkg/tools"
)

func main() {
	fmt.Println("🚀 กำลังเริ่มต้นทดสอบระบบ Facebook Playwright...")
	
	fbTool := tools.NewFacebookTool()
	
	// เตรียม parameter สำหรับลองโพสต์
	args := map[string]any{
		"action":   "post",
		"message":  "สวัสดีโลก! นี่คือการทดสอบโพสต์อัตโนมัติจาก Kaidos (Private Session) 🤖🚀",
		"headless": false, // ตั้งเป็น false เพื่อให้เห็นหน้าจอตอนโหลด Feed
	}
	
	fmt.Println("⏳ กำลังเรียกใช้งาน Facebook Tool (Action: save_session)")
	fmt.Println("💻 ตัวสคริปต์กำลังใช้ Playwright ไปเปิดเว็บบราวเซอร์...")
	fmt.Println("👉 คำแนะนำ: เมื่อหน้าต่างโผล่ขึ้นมา กรุณาล็อกอินและรอสักครู่ตัว Tool จะถ่าย Screenshot และเซฟ Session ให้เอง (อย่าเพิ่งรีบปิดจนกว่าจะเสร็จ)")
	
	// เรียกใช้ Tool ตามแบบฉบับของ Kaidos Pipeline
	ctx := context.Background()
	result := fbTool.Execute(ctx, args)
	
	// แสดงผลลัพธ์
	if result.IsError {
		fmt.Printf("\n❌ เกิดข้อผิดพลาด:\n%s\n", result.ForLLM)
	} else {
		fmt.Printf("\n✅ ผลลัพธ์สำเร็จ:\n%s\n", result.ForLLM)
	}
}
