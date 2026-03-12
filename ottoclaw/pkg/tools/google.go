package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// getCredentials retrieves GOOGLE_EMAIL and GOOGLE_APP_PASSWORD.
// It checks the agent-specific workspace first, then fallbacks to system env.
func getCredentials(ctx context.Context) (string, string) {
	agentID := ToolAgentID(ctx)
	email := ""
	password := ""

	// 1. Try agent-specific workspace env
	if agentID != "" && agentID != "main" {
		home, _ := os.UserHomeDir()
		// Logic matching instance.go resolveAgentWorkspace
		agentDir := filepath.Join(home, ".ottoclaw", "workspace-"+agentID)
		envPath := filepath.Join(agentDir, "env") // Native worker uses 'env' file
		if _, err := os.Stat(envPath); err == nil {
			if file, err := os.Open(envPath); err == nil {
				defer file.Close()
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "GOOGLE_EMAIL=") {
						email = strings.TrimPrefix(line, "GOOGLE_EMAIL=")
					}
					if strings.HasPrefix(line, "GOOGLE_APP_PASSWORD=") {
						password = strings.TrimPrefix(line, "GOOGLE_APP_PASSWORD=")
					}
				}
			}
		}
	}

	// 2. Fallback to global environment
	if email == "" {
		email = os.Getenv("GOOGLE_EMAIL")
	}
	if password == "" {
		password = os.Getenv("GOOGLE_APP_PASSWORD")
	}

	return email, password
}

// SiamSendEmailTool — send an email via Gmail SMTP using App Password.
type SiamSendEmailTool struct{}

func (t *SiamSendEmailTool) Name() string {
	return "siam_send_email"
}

func (t *SiamSendEmailTool) Description() string {
	return "Send an email via Gmail SMTP. Requires GOOGLE_EMAIL and GOOGLE_APP_PASSWORD in environment."
}

func (t *SiamSendEmailTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"to": map[string]any{
				"type":        "string",
				"description": "Recipient email address",
			},
			"subject": map[string]any{
				"type":        "string",
				"description": "Email subject",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Plain text email body",
			},
		},
		"required": []string{"to", "subject", "body"},
	}
}

func (t *SiamSendEmailTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	to, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)

	from, password := getCredentials(ctx)

	if from == "" || password == "" {
		return ErrorResult("GOOGLE_EMAIL or GOOGLE_APP_PASSWORD not set in environment or agent workspace")
	}

	// Setup SMTP auth
	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")

	// Compose message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	// Send email
	err := smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, []byte(msg))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to send email: %v", err))
	}

	return UserResult(fmt.Sprintf("Email sent successfully to %s", to))
}

// SiamReadEmailsTool — read recent emails from Gmail via IMAP.
type SiamReadEmailsTool struct{}

func (t *SiamReadEmailsTool) Name() string {
	return "siam_read_emails"
}

func (t *SiamReadEmailsTool) Description() string {
	return "Read recent emails from Gmail via IMAP. Requires GOOGLE_EMAIL and GOOGLE_APP_PASSWORD."
}

func (t *SiamReadEmailsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": "Number of emails to fetch (default 3)",
			},
		},
	}
}

func (t *SiamReadEmailsTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	email, password := getCredentials(ctx)
	limitFloat, _ := args["limit"].(float64)
	limit := int(limitFloat)
	if limit <= 0 {
		limit = 3
	}

	if email == "" || password == "" {
		return ErrorResult("GOOGLE_EMAIL or GOOGLE_APP_PASSWORD not set")
	}

	// Connect to server
	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("IMAP connect error: %v", err))
	}
	defer c.Logout()

	// Login
	if err := c.Login(email, password); err != nil {
		return ErrorResult(fmt.Sprintf("IMAP login error: %v", err))
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return ErrorResult(fmt.Sprintf("IMAP select error: %v", err))
	}

	if mbox.Messages == 0 {
		return UserResult("No emails found in INBOX.")
	}

	// Get last N messages
	from := uint32(1)
	if mbox.Messages > uint32(limit) {
		from = mbox.Messages - uint32(limit) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	messages := make(chan *imap.Message, uint32(limit))
	done := make(chan error, 1)
	section := &imap.BodySectionName{}
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}, messages)
	}()

	var results []string
	for msg := range messages {
		r := msg.GetBody(section)
		if r == nil {
			continue
		}

		mr, err := mail.CreateReader(r)
		if err != nil {
			continue
		}

		subject := msg.Envelope.Subject
		fromAddr := ""
		if len(msg.Envelope.From) > 0 {
			fromAddr = msg.Envelope.From[0].Address()
		}

		body := ""
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				break
			}

			switch p.Header.(type) {
			case *mail.InlineHeader:
				b, _ := ioutil.ReadAll(p.Body)
				body = string(b)
			}
		}

		results = append(results, fmt.Sprintf("Subject: %s\nFrom: %s\nBody: %s\n---", subject, fromAddr, body))
	}

	if err := <-done; err != nil {
		return ErrorResult(fmt.Sprintf("IMAP fetch error: %v", err))
	}

	return UserResult(strings.Join(results, "\n"))
}

// SiamListCalendarTool — list upcoming events from Google Calendar via CalDAV.
type SiamListCalendarTool struct{}

func (t *SiamListCalendarTool) Name() string {
	return "siam_list_calendar"
}

func (t *SiamListCalendarTool) Description() string {
	return "List upcoming calendar events via CalDAV. Requires GOOGLE_EMAIL and GOOGLE_APP_PASSWORD."
}

func (t *SiamListCalendarTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *SiamListCalendarTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	email, password := getCredentials(ctx)

	if email == "" || password == "" {
		return ErrorResult("GOOGLE_EMAIL or GOOGLE_APP_PASSWORD not set")
	}

	// Google CalDAV URL
	// https://www.google.com/calendar/dav/YOUR_EMAIL/events/
	url := fmt.Sprintf("https://www.google.com/calendar/dav/%s/events/", email)

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", url, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}

	req.SetBasicAuth(email, password)
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "text/xml")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to connect to CalDAV: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorResult(fmt.Sprintf("CalDAV error: %s", resp.Status))
	}

	// For simplicity in this first version, we just return the raw XML or a success message
	// To parsing XML, we'd need more complex logic. 
	// The user's Python example just read the text.
	return UserResult("Successfully connected to Google Calendar. (Advanced XML parsing pending implementation)")
}

// SiamCreateCalendarEventTool — create a new event in Google Calendar via CalDAV.
type SiamCreateCalendarEventTool struct{}

func (t *SiamCreateCalendarEventTool) Name() string {
	return "siam_create_calendar_event"
}

func (t *SiamCreateCalendarEventTool) Description() string {
	return "Create a new calendar event via CalDAV. Requires GOOGLE_EMAIL, GOOGLE_APP_PASSWORD, title, startTime (ISO8601), and endTime (ISO8601)."
}

func (t *SiamCreateCalendarEventTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Event title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Event description",
			},
			"startTime": map[string]any{
				"type":        "string",
				"description": "Start time in ISO8601 format (e.g., 2026-03-12T10:00:00Z)",
			},
			"endTime": map[string]any{
				"type":        "string",
				"description": "End time in ISO8601 format",
			},
		},
		"required": []string{"title", "startTime", "endTime"},
	}
}

func (t *SiamCreateCalendarEventTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	email, password := getCredentials(ctx)
	title, _ := args["title"].(string)
	desc, _ := args["description"].(string)
	start, _ := args["startTime"].(string)
	end, _ := args["endTime"].(string)

	if email == "" || password == "" {
		return ErrorResult("GOOGLE_EMAIL or GOOGLE_APP_PASSWORD not set")
	}

	// Simple VEVENT generation
	uid := fmt.Sprintf("%d@siam-synapse", os.Getpid())
	vcalendar := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Siam-Synapse//OttoClaw//EN
BEGIN:VEVENT
UID:%s
DTSTAMP:20260312T000000Z
DTSTART:%s
DTEND:%s
SUMMARY:%s
DESCRIPTION:%s
END:VEVENT
END:VCALENDAR`, uid, strings.ReplaceAll(strings.ReplaceAll(start, "-", ""), ":", ""), strings.ReplaceAll(strings.ReplaceAll(end, "-", ""), ":", ""), title, desc)

	// In CalDAV, creating an event is often a PUT to a unique URL
	url := fmt.Sprintf("https://www.google.com/calendar/dav/%s/events/%s.ics", email, uid)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(vcalendar))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}

	req.SetBasicAuth(email, password)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to connect to CalDAV: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrorResult(fmt.Sprintf("CalDAV error: %s", resp.Status))
	}

	return UserResult(fmt.Sprintf("Calendar event '%s' created successfully.", title))
}
