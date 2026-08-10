package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func UpgradeInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	mode := "release"
	if _, err := os.Stat(".git"); err == nil {
		mode = "source"
	}
	version := "v0.1.0"
	if data, err := os.ReadFile("VERSION"); err == nil {
		version = strings.TrimSpace(string(data))
	}
	json.NewEncoder(w).Encode(map[string]any{
		"mode":    mode,
		"version": version,
		"repo":    "https://github.com/rouroumaibing/sounds-great-ai",
	})
}

func UpgradeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Pull bool `json:"pull"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	w.Header().Set("Content-Type", "application/json")
	var logs []string

	isSource := false
	if _, err := os.Stat(".git"); err == nil {
		isSource = true
	}

	if isSource {
		if req.Pull {
			out, err := exec.Command("git", "pull").CombinedOutput()
			logs = append(logs, string(out))
			if err != nil {
				json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "git pull failed", "logs": logs})
				return
			}
		}
		out, err := exec.Command("make", "install").CombinedOutput()
		logs = append(logs, string(out))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "make install failed", "logs": logs})
			return
		}
		out, err = exec.Command("make", "build").CombinedOutput()
		logs = append(logs, string(out))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "make build failed", "logs": logs})
			return
		}
		out, err = exec.Command("go", "build", "-o", "bin/server", "cmd/server/main.go").CombinedOutput()
		logs = append(logs, string(out))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "go build failed", "logs": logs})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Upgrade complete (source mode). Restart the server to apply.", "logs": logs})
	} else {
		platformKey := runtime.GOOS + "-" + runtime.GOARCH
		apiURL := "https://api.github.com/repos/rouroumaibing/sounds-great-ai/releases/latest"
		resp, err := http.Get(apiURL)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to fetch release info: " + err.Error(), "logs": logs})
			return
		}
		defer resp.Body.Close()
		var release struct {
			TagName string `json:"tag_name"`
			Assets  []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to parse release info: " + err.Error(), "logs": logs})
			return
		}
		logs = append(logs, fmt.Sprintf("Latest release: %s", release.TagName))
		var downloadURL string
		for _, asset := range release.Assets {
			if strings.Contains(asset.Name, platformKey) {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
		if downloadURL == "" {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": fmt.Sprintf("no release asset found for platform %s", platformKey), "logs": logs})
			return
		}
		dlResp, err := http.Get(downloadURL)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to download release: " + err.Error(), "logs": logs})
			return
		}
		defer dlResp.Body.Close()
		if err := os.MkdirAll("bin", 0755); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to create bin dir: " + err.Error(), "logs": logs})
			return
		}
		outFile, err := os.Create("bin/server")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to create binary file: " + err.Error(), "logs": logs})
			return
		}
		if _, err := io.Copy(outFile, dlResp.Body); err != nil {
			outFile.Close()
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "failed to write binary: " + err.Error(), "logs": logs})
			return
		}
		outFile.Close()
		os.Chmod("bin/server", 0755)
		logs = append(logs, fmt.Sprintf("Downloaded %s to bin/server", downloadURL))
		json.NewEncoder(w).Encode(map[string]any{"success": true, "message": fmt.Sprintf("Upgrade complete (release mode, %s). Restart the server to apply.", release.TagName), "logs": logs})
	}
}

func SPAHandler(distDir string) http.Handler {
	fs := http.FileServer(http.Dir(distDir))
	cleanDistDir := filepath.Clean(distDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		cleanPath := filepath.Join(distDir, filepath.Clean(r.URL.Path))
		if !strings.HasPrefix(cleanPath, cleanDistDir+string(filepath.Separator)) && cleanPath != cleanDistDir {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(cleanPath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fs.ServeHTTP(w, r)
	})
}
