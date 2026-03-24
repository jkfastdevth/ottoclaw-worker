package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// getDriveService returns an authorized Drive service using a Service Account JSON.
func getDriveService(ctx context.Context) (*drive.Service, error) {
	agentID := ToolAgentID(ctx)
	jsonKey := ""

	// 1. Try agent-specific workspace env for the JSON content or path
	if agentID != "" && agentID != "main" {
		home, _ := os.UserHomeDir()
		appHome := filepath.Join(home, ".picoclaw")
		if h := os.Getenv("PICOCLAW_HOME"); h != "" {
			appHome = h
		} else if h := os.Getenv("OTTOCLAW_HOME"); h != "" {
			appHome = h
		} else if _, err := os.Stat(appHome); err != nil {
			otto := filepath.Join(home, ".ottoclaw")
			if _, e := os.Stat(otto); e == nil {
				appHome = otto
			}
		}
		agentDir := filepath.Join(appHome, "workspace-"+agentID)
		jsonPath := filepath.Join(agentDir, "google-drive-key.json")
		if _, err := os.Stat(jsonPath); err == nil {
			return drive.NewService(ctx, option.WithCredentialsFile(jsonPath))
		}
	}

	// 2. Fallback to global env variable GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON (actual JSON content)
	// or GOOGLE_DRIVE_SERVICE_ACCOUNT_FILE (path to file)
	if path := os.Getenv("GOOGLE_DRIVE_SERVICE_ACCOUNT_FILE"); path != "" {
		return drive.NewService(ctx, option.WithCredentialsFile(path))
	}

	jsonKey = os.Getenv("GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON")
	if jsonKey != "" {
		return drive.NewService(ctx, option.WithCredentialsJSON([]byte(jsonKey)))
	}

	return nil, fmt.Errorf("Google Drive credentials not configured (Service Account JSON required)")
}

// SiamDriveUploadTool — upload a file to Google Drive.
type SiamDriveUploadTool struct{}

func (t *SiamDriveUploadTool) Name() string {
	return "siam_drive_upload"
}

func (t *SiamDriveUploadTool) Description() string {
	return "Upload a local file to Google Drive. Requires GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON or file."
}

func (t *SiamDriveUploadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"localPath": map[string]any{
				"type":        "string",
				"description": "Path to the local file to upload",
			},
			"fileName": map[string]any{
				"type":        "string",
				"description": "Name for the file on Google Drive (optional)",
			},
			"folderId": map[string]any{
				"type":        "string",
				"description": "ID of the parent folder on Google Drive (optional)",
			},
		},
		"required": []string{"localPath"},
	}
}

func (t *SiamDriveUploadTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	localPath, _ := args["localPath"].(string)
	fileName, _ := args["fileName"].(string)
	folderID, _ := args["folderId"].(string)

	if fileName == "" {
		fileName = filepath.Base(localPath)
	}

	srv, err := getDriveService(ctx)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Drive Service error: %v", err))
	}

	f, err := os.Open(localPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to open local file: %v", err))
	}
	defer f.Close()

	driveFile := &drive.File{Name: fileName}
	if folderID != "" {
		driveFile.Parents = []string{folderID}
	}

	res, err := srv.Files.Create(driveFile).Media(f).Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to upload to Drive: %v", err))
	}

	return UserResult(fmt.Sprintf("File '%s' uploaded successfully. ID: %s", fileName, res.Id))
}

// SiamDriveSearchTool — search for files on Google Drive.
type SiamDriveSearchTool struct{}

func (t *SiamDriveSearchTool) Name() string {
	return "siam_drive_search"
}

func (t *SiamDriveSearchTool) Description() string {
	return "Search for files on Google Drive using a query."
}

func (t *SiamDriveSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (e.g., \"name contains 'report'\" or \"'shared_folder_id' in parents\")",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SiamDriveSearchTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	query, _ := args["query"].(string)

	srv, err := getDriveService(ctx)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Drive Service error: %v", err))
	}

	res, err := srv.Files.List().Q(query).Fields("files(id, name, mimeType)").Do()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to search Drive: %v", err))
	}

	if len(res.Files) == 0 {
		return UserResult("No files found matching the query.")
	}

	output := "Found files:\n"
	for _, f := range res.Files {
		output += fmt.Sprintf("- %s (ID: %s, Type: %s)\n", f.Name, f.Id, f.MimeType)
	}

	return UserResult(output)
}

// SiamDriveDownloadTool — download a file from Google Drive.
type SiamDriveDownloadTool struct{}

func (t *SiamDriveDownloadTool) Name() string {
	return "siam_drive_download"
}

func (t *SiamDriveDownloadTool) Description() string {
	return "Download a file from Google Drive to the local workspace."
}

func (t *SiamDriveDownloadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fileId": map[string]any{
				"type":        "string",
				"description": "ID of the file to download",
			},
			"outputPath": map[string]any{
				"type":        "string",
				"description": "Local path where the file will be saved",
			},
		},
		"required": []string{"fileId", "outputPath"},
	}
}

func (t *SiamDriveDownloadTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	fileID, _ := args["fileId"].(string)
	destPath, _ := args["outputPath"].(string)

	srv, err := getDriveService(ctx)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Drive Service error: %v", err))
	}

	resp, err := srv.Files.Get(fileID).Download()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to download from Drive: %v", err))
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create local file: %v", err))
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to write to local file: %v", err))
	}

	return UserResult(fmt.Sprintf("File downloaded successfully to: %s", destPath))
}
