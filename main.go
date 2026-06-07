package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Brand and Configuration Constants
const (
	DefaultAPIURL = "https://aerodesk-hub.vercel.app"
	SymlinkName   = "current_wallpaper"
	MaxCacheDays  = 90
)

// ANSI colors for CLI output formatting
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
)

// Wallpaper schema mapping to wallpapers.json
type Wallpaper struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

type Database map[string]Wallpaper

func main() {
	flag.Usage = func() {
		fmt.Printf("%s=== AeroDesk Workspace Manager (CLI) ===%s\n", ColorCyan, ColorReset)
		fmt.Println("Usage:")
		fmt.Println("  aerodesk list            - List all wallpapers registered on the database")
		fmt.Println("  aerodesk apply <id>      - Download and apply wallpaper by unique ID")
		fmt.Println("  aerodesk status          - Display active wallpaper details and cache parameters")
		fmt.Println("  aerodesk clean [--all]   - Remove cached files older than 90 days (or force clean all)")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		printBrandFooter()
	}

	allCleanFlag := flag.Bool("all", false, "Force delete all cached wallpaper files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	command := strings.ToLower(args[0])

	switch command {
	case "list":
		handleList()
	case "apply":
		if len(args) < 2 {
			logError("Please specify a wallpaper ID. Example: aerodesk apply 4fa0a7d")
			os.Exit(1)
		}
		handleApply(args[1])
	case "status":
		handleStatus()
	case "clean":
		handleClean(*allCleanFlag)
	default:
		logError(fmt.Sprintf("Command '%s' not recognized.", command))
		flag.Usage()
		os.Exit(1)
	}
}

// === COMMAND IMPLEMENTATIONS ===

func handleList() {
	db, err := fetchDatabase()
	if err != nil {
		logError(fmt.Sprintf("Failed to reach registry database: %v", err))
		os.Exit(1)
	}

	cacheDir := getCacheDir()

	fmt.Printf("\n%s%-10s %-40s %-10s %-15s%s\n", ColorCyan, "ID", "TITLE", "FORMAT", "STATUS", ColorReset)
	fmt.Println(strings.Repeat("-", 80))

	for id, wp := range db {
		localPath := filepath.Join(cacheDir, fmt.Sprintf("%s.%s", id, wp.Format))
		status := fmt.Sprintf("%sNot Cached%s", ColorYellow, ColorReset)

		if info, err := os.Stat(localPath); err == nil {
			days := int(time.Since(info.ModTime()).Hours() / 24)
			status = fmt.Sprintf("%sCached (%dd old)%s", ColorGreen, days, ColorReset)
			if days >= MaxCacheDays {
				status = fmt.Sprintf("%sExpired (%dd)%s", ColorRed, days, ColorReset)
			}
		}

		title := wp.Title
		if len(title) > 38 {
			title = title[:35] + "..."
		}

		fmt.Printf("%-10s %-40s %-10s %-15s\n", id, title, strings.ToUpper(wp.Format), status)
	}
	printBrandFooter()
}

func handleApply(id string) {
	db, err := fetchDatabase()
	if err != nil {
		logError(fmt.Sprintf("Failed to reach registry database: %v", err))
		os.Exit(1)
	}

	wp, exists := db[id]
	if !exists {
		logError(fmt.Sprintf("No wallpaper found with ID '%s'. Run 'aerodesk list' to check values.", id))
		os.Exit(1)
	}

	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		logError(fmt.Sprintf("Could not create directory structure: %v", err))
		os.Exit(1)
	}

	localPath := filepath.Join(cacheDir, fmt.Sprintf("%s.%s", id, wp.Format))
	needsDownload := true

	if info, err := os.Stat(localPath); err == nil {
		days := int(time.Since(info.ModTime()).Hours() / 24)
		if days >= MaxCacheDays {
			logWarning(fmt.Sprintf("Cached resource is %d days old. Retrying fresh mirror download...", days))
		} else {
			logSuccess(fmt.Sprintf("Valid local asset found (%d days old). Skipping network download.", days))
			needsDownload = false
			currentTime := time.Now()
			_ = os.Chtimes(localPath, currentTime, currentTime)
		}
	}

	if needsDownload {
		logInfo(fmt.Sprintf("Downloading active resource: %s", wp.Title))
		if err := downloadFile(wp.URL, localPath); err != nil {
			logError(fmt.Sprintf("Download failed: %v", err))
			os.Exit(1)
		}
		logSuccess("Download completed.")
	}

	// Symlink generation
	symlinkPath := filepath.Join(cacheDir, SymlinkName)
	_ = os.Remove(symlinkPath) // Clear legacy pointer
	if err := os.Symlink(localPath, symlinkPath); err != nil {
		logError(fmt.Sprintf("Could not write brand symlink: %v", err))
		os.Exit(1)
	}
	logSuccess(fmt.Sprintf("AeroDesk symlink updated to: %s", localPath))

	// Cross-Platform Active Application Hook
	applyWallpaperOS(localPath)
}

func applyWallpaperOS(localPath string) {
	switch runtime.GOOS {
	case "linux":
		ensureDependencies()
		configureHyprlandAutostart(localPath)
		applyLiveWallpaper(localPath)
	case "darwin":
		logInfo("Applying background on macOS...")
		script := fmt.Sprintf(`tell application "Finder" to set desktop picture to POSIX file "%s"`, localPath)
		cmd := exec.Command("osascript", "-e", script)
		if err := cmd.Run(); err != nil {
			logError(fmt.Sprintf("Failed to apply macOS background: %v", err))
		} else {
			logSuccess("macOS desktop background updated.")
		}
	case "windows":
		logInfo("Applying background on Windows...")
		// Change wallpaper via win32 SystemParametersInfo function run inside PowerShell
		psCommand := fmt.Sprintf(`Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class Wallpaper { [DllImport("user32.dll", CharSet=CharSet.Auto)] public static extern int SystemParametersInfo(int uAction, int uParam, string lpvParam, int fuWinIni); }'; [Wallpaper]::SystemParametersInfo(20, 0, '%s', 3)`, localPath)
		cmd := exec.Command("powershell", "-Command", psCommand)
		if err := cmd.Run(); err != nil {
			logError(fmt.Sprintf("Failed to update Windows background API: %v", err))
		} else {
			logSuccess("Windows desktop background updated.")
		}
	}
}

func handleStatus() {
	cacheDir := getCacheDir()
	symlinkPath := filepath.Join(cacheDir, SymlinkName)

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		logWarning("No active live wallpaper config found. Set one via 'aerodesk apply <id>'.")
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		logError("Active workspace pointer is broken.")
		return
	}

	days := int(time.Since(info.ModTime()).Hours() / 24)
	fmt.Printf("\n%s=== AeroDesk Workspace Metrics ===%s\n", ColorCyan, ColorReset)
	fmt.Printf("Active Resource   : %s\n", filepath.Base(target))
	fmt.Printf("File Destination  : %s\n", target)
	fmt.Printf("Cached Lifespan   : %d days %s\n", days, getAgeStatus(days))
	if runtime.GOOS == "linux" {
		fmt.Printf("Engine Status     : Daemon Active (%v)\n", isProcessRunning("mpvpaper"))
	}
	printBrandFooter()
}

func handleClean(forceAll bool) {
	cacheDir := getCacheDir()
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		logError(fmt.Sprintf("Could not scan AeroDesk storage directory: %v", err))
		return
	}

	removedCount := 0
	for _, file := range files {
		if file.Name() == SymlinkName {
			continue
		}

		filePath := filepath.Join(cacheDir, file.Name())
		info, err := file.Info()
		if err != nil {
			continue
		}

		days := int(time.Since(info.ModTime()).Hours() / 24)
		if forceAll || days >= MaxCacheDays {
			if err := os.Remove(filePath); err == nil {
				logInfo(fmt.Sprintf("Removed expired resource: %s (%d days old)", file.Name(), days))
				removedCount++
			}
		}
	}

	logSuccess(fmt.Sprintf("Cleanup complete. Removed %d files from cache.", removedCount))
}

// === FILE SYSTEM & API ENGINE ===

func getCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./aerodesk_cache"
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "AeroDesk")
	}
	return filepath.Join(home, ".local", "share", "backgrounds", "aerodesk")
}

func getAPIURL() string {
	if val, ok := os.LookupEnv("AERODESK_API_URL"); ok {
		return val
	}
	return DefaultAPIURL
}

func fetchDatabase() (Database, error) {
	url := fmt.Sprintf("%s/wallpapers.json", getAPIURL())
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server responded with status: %s", resp.Status)
	}

	var db Database
	if err := json.NewDecoder(resp.Body).Decode(&db); err != nil {
		return nil, err
	}
	return db, nil
}

func downloadFile(url string, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cdn server returned status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func ensureDependencies() {
	deps := []string{"jq", "mpvpaper"}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			logWarning(fmt.Sprintf("Missing package dependency: %s. Resolving...", dep))
			if dep == "mpvpaper" {
				installAURPackage("mpvpaper")
			} else {
				installPacmanPackage(dep)
			}
		}
	}
}

func installPacmanPackage(pkg string) {
	cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func installAURPackage(pkg string) {
	var helper string
	if _, err := exec.LookPath("yay"); err == nil {
		helper = "yay"
	} else if _, err := exec.LookPath("paru"); err == nil {
		helper = "paru"
	} else {
		logError("AUR helper missing. Please install 'mpvpaper' manually.")
		return
	}

	cmd := exec.Command(helper, "-S", "--noconfirm", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func configureHyprlandAutostart(symlinkPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	hyprConfig := filepath.Join(home, ".config/hypr/hyprland.conf")
	if _, err := os.Stat(hyprConfig); os.IsNotExist(err) {
		return
	}

	content, err := os.ReadFile(hyprConfig)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	var updatedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "mpvpaper") && !strings.Contains(trimmed, "current_wallpaper") && !strings.Contains(trimmed, "# Dynamic Live Wallpaper Manager") {
			updatedLines = append(updatedLines, line)
		}
	}

	monitor := "eDP-1"
	if out, err := exec.Command("sh", "-c", "hyprctl monitors | grep 'Monitor' | awk '{print $2}' | head -n 1").Output(); err == nil {
		m := strings.TrimSpace(string(out))
		if m != "" {
			monitor = m
		}
	}

	autostartLine := fmt.Sprintf("exec-once = mpvpaper -o \"--loop-file=inf --no-audio --hwdec=auto\" %s %s", monitor, symlinkPath)
	updatedLines = append(updatedLines, "\n# Dynamic Live Wallpaper Manager", autostartLine)

	_ = os.WriteFile(hyprConfig, []byte(strings.Join(updatedLines, "\n")), 0644)
	logSuccess("AeroDesk autostart configurations linked in hyprland.conf.")
}

func applyLiveWallpaper(symlinkPath string) {
	_ = exec.Command("killall", "mpvpaper").Run()
	time.Sleep(200 * time.Millisecond)

	monitor := "eDP-1"
	if out, err := exec.Command("sh", "-c", "hyprctl monitors | grep 'Monitor' | awk '{print $2}' | head -n 1").Output(); err == nil {
		m := strings.TrimSpace(string(out))
		if m != "" {
			monitor = m
		}
	}

	cmd := exec.Command("mpvpaper", "-o", "--loop-file=inf --no-audio --hwdec=auto", monitor, symlinkPath)
	if err := cmd.Start(); err != nil {
		logError(fmt.Sprintf("Failed to spawn mpvpaper rendering thread: %v", err))
		return
	}

	logSuccess("AeroDesk live background applied.")
}

// === SYSTEM LOGGING & CROSS PROMOTIONS ===

func printBrandFooter() {
	fmt.Println()
	fmt.Printf("%s────────────────────────────────────────────────────────────────────────%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s                             AeroDesk Suite                             %s\n", ColorCyan, ColorReset)
	fmt.Printf("%s────────────────────────────────────────────────────────────────────────%s\n", ColorCyan, ColorReset)
	fmt.Printf(" • %sMaazDB%s      - Embeddable high-performance database engine.\n", ColorBlue, ColorReset)
	fmt.Printf(" • %sMellow Lab%s  - Exploratory research studio in cognitive machine learning.\n", ColorBlue, ColorReset)
	fmt.Printf(" Visit the developer workspace at: %shttps://aerodesk-hub.vercel.app%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s────────────────────────────────────────────────────────────────────────%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

func isProcessRunning(name string) bool {
	cmd := exec.Command("pgrep", name)
	err := cmd.Run()
	return err == nil
}

func getAgeStatus(days int) string {
	if days >= MaxCacheDays {
		return fmt.Sprintf("%s[EXPIRED Cache - Clean Recommended]%s", ColorRed, ColorReset)
	}
	return fmt.Sprintf("%s[Active]%s", ColorGreen, ColorReset)
}

func logInfo(msg string)    { fmt.Printf("%s[*] %s%s\n", ColorBlue, msg, ColorReset) }
func logWarning(msg string) { fmt.Printf("%s[~] %s%s\n", ColorYellow, msg, ColorReset) }
func logSuccess(msg string) { fmt.Printf("%s[✔] %s%s\n", ColorGreen, msg, ColorReset) }
func logError(msg string)   { fmt.Printf("%s[!] Error: %s%s\n", ColorRed, msg, ColorReset) }
