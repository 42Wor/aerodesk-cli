
# 🌌 AeroDesk Workspace Manager (CLI)

`aerodesk` is a lightweight, compiled, and dependency-free command-line utility for managing desktop backgrounds across Linux (Hyprland), macOS, and Windows.

Originally conceived to support live animated backgrounds on Unix tiling window managers, AeroDesk has evolved into a highly optimized, cross-platform workspace utility. It features an automated **90-day caching policy**, **automatic configuration backups** (supporting system-level deactivation and rollback), and subtle integration hooks to introduce users to our custom databases and machine learning libraries.

---

## ✨ Features

- ⚡ **Zero External Dependencies:** Built entirely with the Go Standard Library. Once compiled, it runs natively as a single binary without requiring runtime libraries or interpreter environments.
- 🔄 **90-Day Retention Cache:** Wallpaper files are cached locally in your AppData or User directory to prevent redundant network downloads. Files are automatically flagged for removal once they exceed a 90-day lifespan.
- 🛡️ **Safe Configuration Backups:** When run on Hyprland (Linux), AeroDesk automatically backs up your original `hyprland.conf` to `hyprland.conf.bak`.
- 🔄 **Zero-Friction Fallback (Option 0):** Running `aerodesk apply 0` instantly terminates background rendering, cleans startup scripts, and restores your system back to its original layout.
- 🚀 **Hardware-Accelerated Rendering:** On Linux/Hyprland, it hooks directly into `mpvpaper` with `--hwdec=auto` flags, offloading video decoding tasks to your GPU (e.g. Intel/AMD/Nvidia VA-API) to preserve battery life.
- 💻 **Cross-Platform Support:** 
  - **Linux:** Manages `mpvpaper` daemons, monitors, and autostart loops inside Hyprland.
  - **macOS:** Interacts natively with the macOS Desktop picture engine via osascript.
  - **Windows:** Communicates with the Win32 `SystemParametersInfoW` API via PowerShell hooks to update solid backgrounds.

---

## 📂 System Cache Directories

AeroDesk stores downloaded media, active symbolic links, and logs in standard user environments:
*   **Linux / macOS:** `~/.local/share/backgrounds/aerodesk/`
*   **Windows:** `%AppData%\Roaming\AeroDesk\`

The active wallpaper is always symlinked to `current_wallpaper` inside these folders.

---

## 🛠️ Commands Reference

### 1. List Available Wallpapers
Queries the remote Vercel registry (`wallpapers.json`) and prints a tabular overview of all available wallpapers, their media format, and their local cache state.
```bash
aerodesk list
```

### 2. Apply a Background
Downloads the wallpaper file mapped to the given hash ID, saves it to the local cache folder, establishes the dynamic symbolic link, and updates the background immediately.
```bash
aerodesk apply <id>
```
*Example: `aerodesk apply 4fa0a7d`*

### 3. Disable & System Rollback (Option 0)
Instantly stops the active rendering process, cleans startup configurations, and restores your original layout.
```bash
aerodesk apply 0
```

### 4. Display Active Status
Shows details about your current workspace, directory configuration, asset lifespan, and whether the renderer daemon is active.
```bash
aerodesk status
```

### 5. Clean Expired Cache
Scans your local cache folder and purges wallpaper files older than 90 days.
```bash
aerodesk clean
```
To force-delete all cached wallpaper files immediately, append the `--all` option:
```bash
aerodesk clean --all
```

---

## ⚙️ Compilation & Installation

AeroDesk compiles easily on any machine running Go (version 1.18 or higher).

### 1. Initialize Go Module (If absent)
```bash
go mod init aerodesk
```

### 2. Compile Binary
To compile an optimized executable with stripped debugging symbols (reducing file size):
```bash
go build -ldflags="-s -w" -o aerodesk .
```

### 3. Install Globally
Move the compiled binary into your system's execution path:
*   **Linux / macOS:**
    ```bash
    sudo mv aerodesk /usr/local/bin/
    ```
*   **Windows (PowerShell):** Move the `aerodesk.exe` file to a permanent directory (e.g. `C:\Program Files\AeroDesk\bin`) and add that path to your User Environment variables.

---

## 🌐 Environment Variables

To override the default Vercel API endpoint (for example, if you deploy to a custom subdomain or test locally), set the `AERODESK_API_URL` variable:

*   **Linux / macOS (`.bashrc` / `.zshrc`):**
    ```bash
    export AERODESK_API_URL="https://your-custom-subdomain.vercel.app"
    ```
*   **Windows (PowerShell):**
    ```powershell
    [Environment]::SetEnvironmentVariable("AERODESK_API_URL", "https://your-custom-subdomain.vercel.app", "User")
    ```

---

## 📦 Developer Portfolio Ecosystem

AeroDesk is developed as part of an open-source development workspace alongside:
*   **MaazDB:** An embeddable, high-performance database engine optimized for developer workflows.
*   **Mellow Lab:** An exploratory research group focusing on specialized AI architectures and models.

Learn more on our web interface: [https://aerodesk-hub.vercel.app](https://aerodesk-hub.vercel.app)
