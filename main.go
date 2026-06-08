package main

import (
	"bufio"
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

// Config structure stored inside getCacheDir()/config.json
type Config struct {
	Monitors []string `json:"monitors"` // "all" triggers dynamic auto-detection
	MpvArgs  string   `json:"mpv_args"`
}

func main() {
	flag.Usage = func() {
		fmt.Printf("%s=== AeroDesk Workspace Manager (CLI) ===%s\n", ColorCyan, ColorReset)
		fmt.Println("Usage:")
		fmt.Println("  aerodesk list                      - List all wallpapers registered on the database")
		fmt.Println("  aerodesk apply <id>                - Download and apply wallpaper by unique ID (or '0' to disable)")
		fmt.Println("  aerodesk status                    - Display active wallpaper details and cache parameters")
		fmt.Println("  aerodesk clean [--all]             - Remove cached files older than 90 days (or force clean all)")
		fmt.Println("  aerodesk monitors                  - List out all active system monitors detected")
		fmt.Println("  aerodesk config                    - Launch interactive configuration wizard")
		fmt.Println("  aerodesk config [--monitor=...] [--mpv-opts=...] - Update configuration programmatically via CLI flags")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		printBrandFooter()
	}

	allCleanFlag := flag.Bool("all", false, "Force delete all cached wallpaper files")
	monitorFlag := flag.String("monitor", "", "Set comma-separated monitor names (e.g. 'eDP-1,DP-1' or 'all')")
	mpvOptsFlag := flag.String("mpv-opts", "", "Set custom MPV options")
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
			logError("Please specify a wallpaper ID. Example: aerodesk apply 4fa0a7d (or '0' to disable)")
			os.Exit(1)
		}
		handleApply(args[1])
	case "status":
		handleStatus()
	case "clean":
		handleClean(*allCleanFlag)
	case "monitors":
		handleMonitors()
	case "config":
		handleConfig(*monitorFlag, *mpvOptsFlag)
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
	// Fallback/Disable Switch (Option 0)
	if id == "0" {
		handleDisable()
		return
	}

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
	applyWallpaperOS(symlinkPath)
}

func handleDisable() {
	logInfo("Initiating fallback sequence. Disabling AeroDesk workspace engine...")

	// Kill active video engines
	if runtime.GOOS == "linux" {
		_ = exec.Command("killall", "mpvpaper").Run()
		time.Sleep(200 * time.Millisecond)

		home, err := os.UserHomeDir()
		if err == nil {
			hyprConfig := filepath.Join(home, ".config/hypr/hyprland.conf")
			backupConfig := filepath.Join(home, ".config/hypr/hyprland.conf.bak")

			// Restore original backup if present
			if _, err := os.Stat(backupConfig); err == nil {
				_ = os.Remove(hyprConfig)
				err = os.Rename(backupConfig, hyprConfig)
				if err == nil {
					logSuccess("Original hyprland.conf configuration restored from backup.")
				} else {
					logError(fmt.Sprintf("Failed to restore backup: %v", err))
				}
			} else {
				// Fallback to sanitizing lines manually
				content, err := os.ReadFile(hyprConfig)
				if err == nil {
					lines := strings.Split(string(content), "\n")
					var updatedLines []string
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if !strings.Contains(trimmed, "mpvpaper") && !strings.Contains(trimmed, "current_wallpaper") && !strings.Contains(trimmed, "# Dynamic Live Wallpaper Manager") {
							updatedLines = append(updatedLines, line)
						}
					}
					_ = os.WriteFile(hyprConfig, []byte(strings.Join(updatedLines, "\n")), 0644)
					logSuccess("AeroDesk configurations removed from hyprland.conf.")
				}
			}
		}
	} else if runtime.GOOS == "windows" {
		// Restore default solid background on Windows
		psCommand := `Add-Type -TypeDefinition "using System; using System.Runtime.InteropServices; public class Wallpaper { [DllImport(\"user32.dll\", CharSet=CharSet.Auto)] public static extern int SystemParametersInfo(int uAction, int uParam, string lpvParam, int fuWinIni); }"; [Wallpaper]::SystemParametersInfo(20, 0, \"\", 3)`
		_ = exec.Command("powershell", "-Command", psCommand).Run()
		logSuccess("AeroDesk bypassed. Windows background restored.")
	}

	logSuccess("AeroDesk successfully deactivated. Original system background is restored!")
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
		if file.Name() == SymlinkName || file.Name() == "config.json" {
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

func handleMonitors() {
	fmt.Printf("\n%s=== Active Monitor Layout List ===%s\n", ColorCyan, ColorReset)
	monitors := getMonitors()
	if len(monitors) == 1 && monitors[0] == "*" {
		logWarning("No active Hyprland environment detected. Listing falls back to wildcards.")
		fmt.Println("  [*] Wildcard (renders automatically on all outputs)")
	} else {
		for i, monitor := range monitors {
			fmt.Printf("  [%d] %s\n", i+1, monitor)
		}
	}
	printBrandFooter()
}

func handleConfig(monitorsOpt string, mpvOpts string) {
	// If no arguments/flags are passed via the CLI, launch the interactive configuration wizard
	if monitorsOpt == "" && mpvOpts == "" {
		runConfigWizard()
		return
	}

	cfg := loadConfig()
	updated := false

	if monitorsOpt != "" {
		if monitorsOpt == "all" || monitorsOpt == "default" {
			cfg.Monitors = []string{"all"}
		} else {
			parts := strings.Split(monitorsOpt, ",")
			var cleanParts []string
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					cleanParts = append(cleanParts, trimmed)
				}
			}
			cfg.Monitors = cleanParts
		}
		updated = true
		logSuccess(fmt.Sprintf("Monitors updated in config: %v", cfg.Monitors))
	}

	if mpvOpts != "" {
		cfg.MpvArgs = mpvOpts
		updated = true
		logSuccess(fmt.Sprintf("MPV arguments updated in config: %s", cfg.MpvArgs))
	}

	if updated {
		if err := saveConfig(cfg); err != nil {
			logError(fmt.Sprintf("Failed to save configuration settings: %v", err))
		}
	} else {
		fmt.Printf("\n%s=== AeroDesk Engine Configurations ===%s\n", ColorCyan, ColorReset)
		fmt.Printf("Configured Monitors : %s\n", strings.Join(cfg.Monitors, ", "))
		fmt.Printf("MPV Arguments Flags : %s\n", cfg.MpvArgs)
		fmt.Printf("Storage Location    : %s\n", filepath.Join(getCacheDir(), "config.json"))
		printBrandFooter()
	}
}

func runConfigWizard() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s=== AeroDesk Interactive Configuration Wizard ===%s\n", ColorCyan, ColorReset)

	// Step 1: Monitor Choice Configuration
	fmt.Printf("\n%s[Step 1/2] Monitor Configuration Selection%s\n", ColorBlue, ColorReset)
	detected := getMonitors()
	fmt.Println("Detected active monitors on your system:")
	if len(detected) == 1 && detected[0] == "*" {
		fmt.Println("  No active Hyprland monitors found. Using fallback '*' (renders on all displays).")
	} else {
		for i, m := range detected {
			fmt.Printf("  [%d] %s\n", i+1, m)
		}
	}

	fmt.Println("\nConfiguration Options:")
	fmt.Println("  - Type 'all' to dynamically target all system screens at runtime.")
	fmt.Println("  - Type specific monitor names separated by commas (e.g., 'eDP-1,DP-2').")
	if !(len(detected) == 1 && detected[0] == "*") {
		fmt.Println("  - Type the numbers of the monitors you want to target (e.g., '1' or '1,2').")
	}

	fmt.Print("\nYour selection [default: all]: ")
	monInput, _ := reader.ReadString('\n')
	monInput = strings.TrimSpace(monInput)

	var selectedMonitors []string
	if monInput == "" || strings.ToLower(monInput) == "all" {
		selectedMonitors = []string{"all"}
	} else {
		parts := strings.Split(monInput, ",")
		isNumericSelection := true
		for _, p := range parts {
			p = strings.TrimSpace(p)
			var num int
			_, err := fmt.Sscanf(p, "%d", &num)
			if err != nil || num < 1 || num > len(detected) {
				isNumericSelection = false
				break
			}
		}

		if isNumericSelection && !(len(detected) == 1 && detected[0] == "*") {
			for _, p := range parts {
				p = strings.TrimSpace(p)
				var num int
				_, _ = fmt.Sscanf(p, "%d", &num)
				selectedMonitors = append(selectedMonitors, detected[num-1])
			}
		} else {
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					selectedMonitors = append(selectedMonitors, trimmed)
				}
			}
		}
	}

	// Step 2: MPV Arguments/Optimization selection
	fmt.Printf("\n%s[Step 2/2] Performance Profile Selection%s\n", ColorBlue, ColorReset)
	fmt.Println("Choose an MPV background rendering performance profile:")
	fmt.Println("  [1] Balanced (Default: hardware decoding enabled, silent, looped)")
	fmt.Println("  [2] Low GPU Power (Optimized for battery, skips frames if slow, fast decode)")
	fmt.Println("  [3] Custom (Manually enter your own preferred MPV flags)")

	fmt.Print("\nYour selection [default: 1]: ")
	profInput, _ := reader.ReadString('\n')
	profInput = strings.TrimSpace(profInput)

	selectedMpvArgs := "--loop-file=inf --no-audio --hwdec=auto"
	switch profInput {
	case "2":
		selectedMpvArgs = "--loop-file=inf --no-audio --hwdec=auto --vd-lavc-fast --framedrop=vo"
	case "3":
		fmt.Print("Enter your custom MPV rendering arguments: ")
		customArgs, _ := reader.ReadString('\n')
		customArgs = strings.TrimSpace(customArgs)
		if customArgs != "" {
			selectedMpvArgs = customArgs
		}
	}

	// Save configuration object
	cfg := Config{
		Monitors: selectedMonitors,
		MpvArgs:  selectedMpvArgs,
	}

	if err := saveConfig(cfg); err != nil {
		logError(fmt.Sprintf("Failed to write config file: %v", err))
	} else {
		fmt.Printf("\n%s[✔] Configuration successfully saved!%s\n", ColorGreen, ColorReset)
		fmt.Printf("Target Monitors : %v\n", cfg.Monitors)
		fmt.Printf("MPV Arguments   : %s\n", cfg.MpvArgs)
	}
	printBrandFooter()
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

func loadConfig() Config {
	cfgPath := filepath.Join(getCacheDir(), "config.json")
	defaultConfig := Config{
		Monitors: []string{"all"},
		MpvArgs:  "--loop-file=inf --no-audio --hwdec=auto",
	}

	file, err := os.Open(cfgPath)
	if err != nil {
		return defaultConfig
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return defaultConfig
	}
	return cfg
}

func saveConfig(cfg Config) error {
	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	cfgPath := filepath.Join(cacheDir, "config.json")
	file, err := os.Create(cfgPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func getMonitors() []string {
	var monitors []string

	// Attempt structured JSON fetch from Hyprland
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err == nil {
		type HyprMonitor struct {
			Name string `json:"name"`
		}
		var hyprMonitors []HyprMonitor
		if err := json.Unmarshal(out, &hyprMonitors); err == nil {
			for _, m := range hyprMonitors {
				if m.Name != "" {
					monitors = append(monitors, m.Name)
				}
			}
		}
	}

	// Plain text grep/awk parsing fallback if JSON fails
	if len(monitors) == 0 {
		out, err := exec.Command("sh", "-c", "hyprctl monitors | grep 'Monitor' | awk '{print $2}'").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					monitors = append(monitors, trimmed)
				}
			}
		}
	}

	// Asterisk fallback (renders automatically on all active outputs if detection fails)
	if len(monitors) == 0 {
		monitors = []string{"*"}
	}
	return monitors
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

	backupConfig := filepath.Join(home, ".config/hypr/hyprland.conf.bak")
	// Safely backup original configuration once
	if _, err := os.Stat(backupConfig); os.IsNotExist(err) {
		_ = copyFile(hyprConfig, backupConfig)
		logSuccess("Created backup copy of original configuration at hyprland.conf.bak")
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

	cfg := loadConfig()
	var monitors []string
	if len(cfg.Monitors) == 1 && cfg.Monitors[0] == "all" {
		monitors = getMonitors()
	} else {
		monitors = cfg.Monitors
	}

	updatedLines = append(updatedLines, "\n# Dynamic Live Wallpaper Manager")
	for _, monitor := range monitors {
		autostartLine := fmt.Sprintf("exec-once = mpvpaper -o %q %s %s", cfg.MpvArgs, monitor, symlinkPath)
		updatedLines = append(updatedLines, autostartLine)
	}

	_ = os.WriteFile(hyprConfig, []byte(strings.Join(updatedLines, "\n")), 0644)
	logSuccess("AeroDesk autostart configurations linked in hyprland.conf.")
}

func applyLiveWallpaper(symlinkPath string) {
	_ = exec.Command("killall", "mpvpaper").Run()
	time.Sleep(200 * time.Millisecond)

	cfg := loadConfig()
	var monitors []string
	if len(cfg.Monitors) == 1 && cfg.Monitors[0] == "all" {
		monitors = getMonitors()
	} else {
		monitors = cfg.Monitors
	}

	for _, monitor := range monitors {
		cmd := exec.Command("mpvpaper", "-o", cfg.MpvArgs, monitor, symlinkPath)
		if err := cmd.Start(); err != nil {
			logError(fmt.Sprintf("Failed to spawn mpvpaper rendering thread on %s: %v", monitor, err))
		} else {
			logSuccess(fmt.Sprintf("AeroDesk live background applied to monitor: %s", monitor))
		}
	}
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func logInfo(msg string)    { fmt.Printf("%s[*] %s%s\n", ColorBlue, msg, ColorReset) }
func logWarning(msg string) { fmt.Printf("%s[~] %s%s\n", ColorYellow, msg, ColorReset) }
func logSuccess(msg string) { fmt.Printf("%s[✔] %s%s\n", ColorGreen, msg, ColorReset) }
func logError(msg string)   { fmt.Printf("%s[!] Error: %s%s\n", ColorRed, msg, ColorReset) }
