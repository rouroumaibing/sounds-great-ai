package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sounds-great-ai/web"
)

// Injected at build time via -ldflags:
//
//	-X main.version=$(git describe --tags --always --dirty)
//	-X main.embeddedBuildID=$(cat web/dist/.build-id)
//
// version feeds /api/upgrade/info (the frontend polls it to offer a refresh
// after a redeploy); embeddedBuildID ranks the embedded frontend against the
// on-disk web/dist so SPAHandler serves whichever is newer.
var (
	version         = "dev"
	embeddedBuildID = "0"
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
	ver := version
	if ver == "dev" {
		// No -ldflags injection (plain `go run`/`go build`): fall back to a
		// VERSION file, keeping the historical behavior.
		if data, err := os.ReadFile("VERSION"); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				ver = v
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"mode":    mode,
		"version": ver,
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
		out, err = exec.Command("go", "build", "-tags", "embeddist", "-ldflags", goBuildLdflags(), "-o", "bin/sounds-great-ai", "cmd/server/main.go").CombinedOutput()
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
		binName := "bin/sounds-great-ai"
		if runtime.GOOS == "windows" {
			binName = "bin/sounds-great-ai.exe"
		}
		outFile, err := os.Create(binName)
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
		os.Chmod(binName, 0755)
		logs = append(logs, fmt.Sprintf("Downloaded %s to %s", downloadURL, binName))
		json.NewEncoder(w).Encode(map[string]any{"success": true, "message": fmt.Sprintf("Upgrade complete (release mode, %s). Restart the server to apply.", release.TagName), "logs": logs})
	}
}

// goBuildLdflags mirrors the Makefile's build flags for the in-app source-mode
// upgrade path: stamp the git version and the frontend build id into the
// binary so /api/upgrade/info and SPAHandler ranking stay correct.
func goBuildLdflags() string {
	ver := "dev"
	if out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			ver = v
		}
	}
	buildID := "0"
	if data, err := os.ReadFile(filepath.Join("web", "dist", ".build-id")); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			buildID = v
		}
	}
	return fmt.Sprintf("-X main.version=%s -X main.embeddedBuildID=%s", ver, buildID)
}

// spaFS is one servable frontend tree: the on-disk web/dist, the previous
// build kept at web/dist.old (grace window for tabs still holding old chunk
// hashes), or the frontend snapshot embedded at compile time.
type spaFS struct {
	fsys fs.FS // rooted at the dist tree; nil when unavailable
	id   int64 // build id; the larger id wins when disk and embedded coexist
}

func diskSpaFS(dir string) spaFS {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return spaFS{}
	}
	return spaFS{fsys: os.DirFS(dir), id: diskBuildID(dir)}
}

// diskBuildID prefers the .build-id written by the vite build; the index.html
// mtime fallback keeps plain `npm run build` output (no .build-id) rankable
// by freshness too.
func diskBuildID(dir string) int64 {
	if b, err := os.ReadFile(filepath.Join(dir, ".build-id")); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
			return v
		}
	}
	if fi, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		return fi.ModTime().Unix()
	}
	return 0
}

func embeddedSpaFS() spaFS {
	sub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return spaFS{}
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return spaFS{}
	}
	id, _ := strconv.ParseInt(embeddedBuildID, 10, 64)
	return spaFS{fsys: sub, id: id}
}

// SPAHandler serves the frontend with redeploy-safe semantics:
//
//   - Missing static assets return a real 404, never an index.html body with
//     a text/html MIME type (module scripts hard-fail on that, which is how
//     stale tabs used to die after every upgrade).
//   - Only navigation requests fall back to index.html.
//   - /assets/* are content-hashed and served immutable; the entry points
//     (index.html, sw.js, manifest) are sent with no-cache so a deploy is
//     picked up on the next reload, not from heuristic browser caching.
//   - When both an on-disk dist and the embedded snapshot exist, the newer
//     build id wins (binary-only upgrades embed a newer frontend; local
//     iteration rebuilds dist without recompiling).
//   - web/dist.old keeps the previous generation's chunks alive so tabs
//     opened before a redeploy can still load them for one more build.
func SPAHandler(workspaceDir string) http.Handler {
	distDir := filepath.Join(workspaceDir, "web", "dist")
	primary, secondary := rankSpaRoots(diskSpaFS(distDir), embeddedSpaFS())
	// The previous on-disk build keeps old hashed chunks alive for one more
	// generation so tabs opened before a redeploy can still load them.
	return spaHandlerFromRoots(primary, secondary, diskSpaFS(distDir+".old"))
}

// rankSpaRoots returns (primary, secondary) with the newer build first: a
// binary-only upgrade embeds a newer frontend than the stale dist on disk,
// while local iteration rebuilds dist without recompiling the binary.
func rankSpaRoots(disk, embedded spaFS) (primary, secondary spaFS) {
	if embedded.fsys != nil && (disk.fsys == nil || embedded.id > disk.id) {
		return embedded, disk
	}
	return disk, embedded
}

func spaHandlerFromRoots(primary, secondary, grace spaFS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		isNavigation := r.Header.Get("Sec-Fetch-Dest") == "document" ||
			strings.Contains(r.Header.Get("Accept"), "text/html")
		if strings.HasPrefix(path.Base(name), ".") {
			// Build metadata (.build-id) is never served.
			if !isNavigation {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}

		for _, root := range []spaFS{primary, secondary, grace} {
			if f, ok := openSpaFile(root, name); ok {
				serveSpaFile(w, r, f, name)
				return
			}
		}
		if isNavigation {
			if f, ok := openSpaFile(primary, "index.html"); ok {
				serveSpaFile(w, r, f, "index.html")
				return
			}
		}
		http.NotFound(w, r)
	})
}

func openSpaFile(root spaFS, name string) (fs.File, bool) {
	if root.fsys == nil || !fs.ValidPath(name) {
		return nil, false
	}
	if fi, err := fs.Stat(root.fsys, name); err != nil || fi.IsDir() {
		return nil, false
	}
	f, err := root.fsys.Open(name)
	if err != nil {
		return nil, false
	}
	return f, true
}

func serveSpaFile(w http.ResponseWriter, r *http.Request, f fs.File, name string) {
	defer f.Close()
	switch {
	case strings.HasPrefix(name, "assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case name == "index.html", name == "sw.js", name == "registerSW.js",
		name == "manifest.json", strings.HasSuffix(name, ".webmanifest"):
		w.Header().Set("Cache-Control", "no-cache")
	}
	var modtime time.Time
	if fi, err := f.Stat(); err == nil {
		modtime = fi.ModTime()
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, path.Base(name), modtime, rs)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	io.Copy(w, f)
}
