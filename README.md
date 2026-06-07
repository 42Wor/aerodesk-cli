
# 🌌 Hyprland Live Wallpaper Manager

An automated, lightweight, and hardware-accelerated live wallpaper manager for Arch Linux and the Hyprland Window Manager. 

Originally built to easily set up the viral *"Furina Cook Pardon"* live wallpaper, this project has evolved into a universal manager for animated wallpapers (MP4, GIF, WEBM, MKV) downloaded from sites like [VSThemes](https://vsthemes.org/) or exported from Wallpaper Engine.

## ✨ Features

- 🧠 **Smart Extraction:** Blindly drop `.rar` or `.zip` files into a folder. The script scans the archives, extracts only the ones containing playable media, organizes them into numbered folders, and automatically deletes the leftover archives.
- 🚀 **Hardware Acceleration:** Uses `mpvpaper` configured with `--hwdec=auto`. This offloads video decoding to your GPU (e.g., Intel UHD Graphics via `vaapi`), keeping CPU usage near 0% and saving laptop battery.
- 🔄 **Live Reloading:** Swapping wallpapers updates your desktop instantly. No need to log out or restart Hyprland.
- 🏷️ **Auto-Tagging:** Automatically reads `project.json` files from VSThemes downloads to display clean, human-readable titles in the terminal menu, along with media type tags (e.g., `[MP4]`, `[GIF]`).
- 🛡️ **Safe Symlinking:** Never clutters your `hyprland.conf`. It creates a single permanent symlink and updates it dynamically in the background.

---

## 📦 Prerequisites

This script is designed for **Arch Linux** running **Hyprland**. 
It will automatically attempt to install missing dependencies, but you should have an AUR helper (`yay` or `paru`) installed.

**Dependencies used:**
- `mpvpaper` (AUR) - For rendering the wallpaper.
- `unrar` & `unzip` (Pacman) - For archive extraction.

---

## 🚀 Installation & Usage

### 1. Clone the Repository
```bash
git clone https://github.com/YOUR_USERNAME/furina-cook-pardon-linux.git
cd furina-cook-pardon-linux
chmod +x *.sh
```

### 2. Initialize the Archive Folder
Run the extraction script once to generate the drop-folder:
```bash
./unzip.sh
```
*This will create a folder named `zip_folders/`.*

### 3. Add Your Wallpapers
Download your favorite live wallpapers (as `.rar` or `.zip` files) and move them into the newly created `zip_folders/` directory.

### 4. Extract & Organize
Run the extraction script again:
```bash
./unzip.sh
```
*The script will scan the archives, extract valid wallpapers into clean numbered folders (1/, 2/, 3/), and delete the original archives to save space.*

### 5. Select and Apply
Run the setup script to open the interactive menu:
```bash
./setup.sh
```
Type the number of the wallpaper you want, press Enter, and watch your desktop come to life!

---

## 🔋 Battery Saving Tip (Highly Recommended)

Because rendering video constantly can drain laptop batteries, it is highly recommended to add keybindings to pause the wallpaper when you have windows maximized or are playing games.

Add these lines to your `~/.config/hypr/hyprland.conf`:

```ini
# Pause live wallpaper (Freezes mpvpaper, drops GPU usage to 0)
bind = SUPER, P, exec, killall -STOP mpvpaper

# Resume live wallpaper
bind = SUPER SHIFT, P, exec, killall -CONT mpvpaper
```

---

## 📂 Folder Structure

```text
.
├── setup.sh          # The interactive wallpaper selector
├── unzip.sh          # The smart archive extractor
├── zip_folders/      # Drop your downloaded .rar/.zip files here
├── 1/                # Auto-generated folder containing Wallpaper 1
├── 2/                # Auto-generated folder containing Wallpaper 2
└── README.md
```

## ⚠️ Note on Wallpaper Engine "Scenes"
If you download a wallpaper that only contains a `scene.pkg` file (a proprietary Wallpaper Engine format) and no actual video file, the `unzip.sh` script will safely ignore it, and `setup.sh` will flag it as `[Scene / No Media]` to prevent crashes.
```

### How to push this to GitHub:
Once you save this file, you can push your final, polished project to GitHub:
```bash
git add README.md
git commit -m "Add comprehensive README documentation"
git push
```