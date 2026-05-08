package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var errHostFolderPickerCanceled = errors.New("folder picker canceled")

func hostBridge(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("agentops host-bridge", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:17321", "host bridge listen address")
	token := fs.String("token", "", "optional API token; defaults to AGENTOPS_MCP_TOKEN when empty")
	if err := fs.Parse(args); err != nil {
		return err
	}
	mux := http.NewServeMux()
	bridgeToken := strings.TrimSpace(*token)
	if bridgeToken == "" {
		bridgeToken = strings.TrimSpace(configuredBridgeToken())
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "server": "agentops-host-bridge"})
	})
	mux.HandleFunc("/pick-folder", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if bridgeToken != "" && r.Header.Get("X-AgentOps-Host-Bridge-Token") != bridgeToken {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		var req struct {
			Title string `json:"title"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req)
		path, err := pickFolderOnHost(req.Title)
		if err != nil {
			if errors.Is(err, errHostFolderPickerCanceled) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "folder_picker_canceled", "error": "folder selection was canceled"})
				return
			}
			writeJSON(w, http.StatusNotImplemented, map[string]any{"code": "folder_picker_unavailable", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": path})
	})
	server := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("AgentOps Host Bridge listening on http://%s", *addr)
	log.Printf("Use AGENTOPS_HOST_BRIDGE_URL=http://host.docker.internal:<port> for Docker API containers.")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func configuredBridgeToken() string {
	// Keep this small helper separate so host-bridge can run without opening a DB connection.
	return strings.TrimSpace(os.Getenv("AGENTOPS_MCP_TOKEN"))
}

func pickFolderOnHost(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Choose folder"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`POSIX path of (choose folder with prompt %q)`, title)
		cmd = exec.CommandContext(ctx, "osascript", "-e", script)
	case "linux":
		if _, err := exec.LookPath("zenity"); err == nil {
			cmd = exec.CommandContext(ctx, "zenity", "--file-selection", "--directory", "--title", title)
		} else if _, err := exec.LookPath("kdialog"); err == nil {
			cmd = exec.CommandContext(ctx, "kdialog", "--getexistingdirectory", ".", title)
		} else {
			return "", errors.New("native folder picker requires zenity or kdialog on Linux")
		}
	case "windows":
		script := `$app = New-Object -ComObject Shell.Application; $folder = $app.BrowseForFolder(0, "` + strings.ReplaceAll(title, `"`, `'`) + `", 0); if ($folder -eq $null) { exit 2 }; $folder.Self.Path`
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	default:
		return "", fmt.Errorf("native folder picker is unsupported on %s", runtime.GOOS)
	}
	output, err := cmd.CombinedOutput()
	selected := strings.TrimSpace(string(output))
	if err != nil {
		if selected == "" || strings.Contains(selected, "-128") || strings.Contains(strings.ToLower(selected), "cancel") {
			return "", errHostFolderPickerCanceled
		}
		return "", fmt.Errorf("native folder picker failed: %s", selected)
	}
	if selected == "" {
		return "", errHostFolderPickerCanceled
	}
	return selected, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
