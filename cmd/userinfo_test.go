package cmd

import "testing"

func TestClassifyLoginSource(t *testing.T) {
	tests := []struct {
		name string
		tty  string
		host string
		want string
	}{
		{name: "ssh", tty: "pts/0", host: "192.0.2.10", want: "ssh"},
		{name: "tmux", tty: "pts/20", host: "tmux(123).%1", want: "tmux"},
		{name: "console", tty: "tty2", host: "", want: "getty/console"},
		{name: "graphical", tty: "seat0", host: "", want: "local graphical"},
		{name: "local terminal", tty: "pts/1", host: "", want: "local terminal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyLoginSource(tt.tty, tt.host); got != tt.want {
				t.Fatalf("classifyLoginSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsRealUser(t *testing.T) {
	uidRange := loginDefRange{uidMin: 1000, uidMax: 60000}

	tests := []struct {
		name  string
		entry passwdEntry
		want  bool
	}{
		{name: "root", entry: passwdEntry{uid: 0, shell: "/bin/bash"}, want: true},
		{name: "normal user", entry: passwdEntry{uid: 1000, shell: "/bin/bash"}, want: true},
		{name: "system user", entry: passwdEntry{uid: 999, shell: "/bin/bash"}, want: false},
		{name: "nologin user", entry: passwdEntry{uid: 1001, shell: "/usr/sbin/nologin"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRealUser(tt.entry, uidRange); got != tt.want {
				t.Fatalf("isRealUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPasswordStatusForUser(t *testing.T) {
	statuses := map[string]string{
		"unlocked": "$y$j9T$hash",
		"locked":   "!$y$j9T$hash",
		"empty":    "",
	}

	tests := []struct {
		name       string
		user       string
		wantStatus string
		wantLocked *bool
	}{
		{name: "unlocked", user: "unlocked", wantStatus: "unlocked", wantLocked: boolPtr(false)},
		{name: "locked", user: "locked", wantStatus: "locked", wantLocked: boolPtr(true)},
		{name: "empty", user: "empty", wantStatus: "empty password", wantLocked: boolPtr(false)},
		{name: "unknown", user: "missing", wantStatus: "unknown", wantLocked: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotLocked := passwordStatusForUser(tt.user, statuses)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if tt.wantLocked == nil {
				if gotLocked != nil {
					t.Fatalf("locked = %v, want nil", *gotLocked)
				}
				return
			}
			if gotLocked == nil || *gotLocked != *tt.wantLocked {
				t.Fatalf("locked = %v, want %v", gotLocked, *tt.wantLocked)
			}
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}
