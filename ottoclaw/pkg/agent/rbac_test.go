package agent

import (
	"testing"
	"github.com/sipeed/ottoclaw/pkg/config"
)

func TestCanExecuteTool(t *testing.T) {
	al := &AgentLoop{}

	tests := []struct {
		name     string
		role     string
		tool     string
		wantErr  bool
	}{
		// Admin can do anything
		{"AdminExec", "admin", "exec", false},
		{"AdminWrite", "admin", "write_file", false},
		{"AdminMessage", "admin", "message", false},
		
		// Trusted can do sensitive things (except if restricted by other means)
		{"TrustedExec", "trusted", "exec", false},
		{"TrustedWrite", "trusted", "write_file", false},
		{"TrustedMessage", "trusted", "message", false},

		// Guest is restricted
		{"GuestExec", "guest", "exec", true},
		{"GuestWrite", "guest", "write_file", true},
		{"GuestEdit", "guest", "edit_file", true},
		{"GuestAppend", "guest", "append_file", true},
		{"GuestSpawn", "guest", "spawn", true},
		{"GuestInstallSkill", "guest", "install_skill", true},
		
		// Guest can use basic tools
		{"GuestMessage", "guest", "message", false},
		{"GuestReadFile", "guest", "read_file", false},
		{"GuestListDir", "guest", "list_dir", false},
		{"GuestWebSearch", "guest", "web_search", false},

		// Empty/Unknown role defaults to Guest
		{"EmptyRoleExec", "", "exec", true},
		{"EmptyRoleMessage", "", "message", false},
		{"UnknownRoleExec", "anonymous", "exec", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AgentInstance{
				Role: tt.role,
			}
			err := al.canExecuteTool(agent, tt.tool)
			if (err != nil) != tt.wantErr {
				t.Errorf("canExecuteTool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveAgentRole(t *testing.T) {
	tests := []struct {
		name     string
		agentRole string
		defaultRole string
		want     string
	}{
		{"BothSet", "admin", "guest", "admin"},
		{"AgentSet", "trusted", "", "trusted"},
		{"DefaultSet", "", "trusted", "trusted"},
		{"NoneSet", "", "", "guest"},
		{"Spaces", "  admin  ", " guest ", "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentCfg := &config.AgentConfig{Role: tt.agentRole}
			defaults := &config.AgentDefaults{Role: tt.defaultRole}
			if got := resolveAgentRole(agentCfg, defaults); got != tt.want {
				t.Errorf("resolveAgentRole() = %v, want %v", got, tt.want)
			}
		})
	}
}
