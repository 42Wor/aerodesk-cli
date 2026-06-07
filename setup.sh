#!/usr/bin/env bash

set -euo pipefail

# ANSI Color Codes
GREEN="\e[32m"
RED="\e[31m"
YELLOW="\e[33m"
BLUE="\e[34m"
RESET="\e[0m"

echo -e "${BLUE}==================================================${RESET}"
echo -e "${BLUE}       Live Wallpaper Selection Manager           ${RESET}"
echo -e "${BLUE}==================================================${RESET}"

# Ensure mpvpaper is ready
if ! command -v mpvpaper &> /dev/null; then
    if command -v yay &> /dev/null; then
        yay -S --noconfirm mpvpaper
    elif command -v paru &> /dev/null; then
        paru -S --noconfirm mpvpaper
    else
        echo -e "${RED}[!] 'mpvpaper' is missing. Please install it manually.${RESET}"
        exit 1
    fi
fi

# Set up local wallpaper store
WALLPAPER_STORE="$HOME/.local/share/backgrounds/live-wallpapers"
SYMLINK_PATH="$WALLPAPER_STORE/current_wallpaper"
mkdir -p "$WALLPAPER_STORE"

DIRS=()
DISPLAY_NAMES=()
PLAYABLE=()
VIDEO_PATHS=()

# 1. Scan directories for any animated format
for d in */; do
    clean_d="${d%/}"
    
    # Ignore our zip dump folder and temp folders
    if [ "$clean_d" != "zip_folders" ] && [ "$clean_d" != "temp_extracted" ]; then
        
        # Find mp4, gif, webm, or mkv (case insensitive)
        VIDEO_PATH=$(find "$clean_d" -type f -iregex '.*\.\(mp4\|gif\|webm\|mkv\)$' | head -n 1)
        
        TITLE=""
        if [ -f "$clean_d/project.json" ]; then
            TITLE=$(grep -oP '"title"\s*:\s*"\K[^"]+' "$clean_d/project.json" || true)
        fi
        [ -z "$TITLE" ] && TITLE="$clean_d"

        DIRS+=("$clean_d")
        if [ -z "$VIDEO_PATH" ]; then
            DISPLAY_NAMES+=("${TITLE} ${RED}[Scene / No Media]${RESET}")
            PLAYABLE+=(0)
            VIDEO_PATHS+=("")
        else
            EXT="${VIDEO_PATH##*.}"
            DISPLAY_NAMES+=("$TITLE ${YELLOW}[${EXT^^}]${RESET}")
            PLAYABLE+=(1)
            VIDEO_PATHS+=("$VIDEO_PATH")
        fi
    fi
done

if [ ${#DIRS[@]} -eq 0 ]; then
    echo -e "${RED}[!] No wallpaper folders found.${RESET}"
    echo -e "    Put your archives in 'zip_folders/' and run ./unzip.sh first."
    exit 1
fi

# 2. Present options (including the new Option 0 Fallback)
echo -e "${BLUE}Available Wallpapers:${RESET}"
for i in "${!DIRS[@]}"; do
    echo -e "  [${GREEN}$((i+1))${RESET}] ${DISPLAY_NAMES[$i]}"
done
echo -e "  [${RED}0${RESET}] Disable Live Wallpaper (Restore Old Static Background)"

echo -ne "\n${YELLOW}Select a wallpaper to set (0-${#DIRS[@]}): ${RESET}"
read -r CHOICE

if ! [[ "$CHOICE" =~ ^[0-9]+$ ]] || [ "$CHOICE" -lt 0 ] || [ "$CHOICE" -gt "${#DIRS[@]}" ]; then
    echo -e "${RED}[!] Invalid choice.${RESET}"
    exit 1
fi

HYPR_CONFIG="$HOME/.config/hypr/hyprland.conf"

# === FALLBACK / DISABLE OPTION ===
if [ "$CHOICE" -eq 0 ]; then
    echo -e "${YELLOW}[*] Disabling live wallpaper...${RESET}"
    if pgrep mpvpaper > /dev/null; then
        killall mpvpaper
        sleep 0.5
    fi
    
    # Clean up the autostart line from hyprland.conf so it boots normally
    if [ -f "$HYPR_CONFIG" ]; then
        sed -i '/mpvpaper/d' "$HYPR_CONFIG"
        sed -i '/# Dynamic Live Wallpaper Manager/d' "$HYPR_CONFIG"
        echo -e "${GREEN}[+] Removed autostart configuration from hyprland.conf.${RESET}"
    fi
    
    echo -e "${GREEN}[✔] Live wallpaper turned off. Your original static background is restored!${RESET}"
    exit 0
fi

# === WALLPAPER SETTING OPTION ===
SELECTED_INDEX=$((CHOICE-1))

if [ "${PLAYABLE[$SELECTED_INDEX]}" -eq 0 ]; then
    echo -e "${RED}[!] Error: Folder '${DIRS[$SELECTED_INDEX]}' does not contain a playable video or GIF.${RESET}"
    exit 1
fi

# Copy file and update symlink
VIDEO_PATH="${VIDEO_PATHS[$SELECTED_INDEX]}"
WALLPAPER_NAME=$(basename "$VIDEO_PATH")

cp "$VIDEO_PATH" "$WALLPAPER_STORE/$WALLPAPER_NAME"
ln -sf "$WALLPAPER_STORE/$WALLPAPER_NAME" "$SYMLINK_PATH"

echo -e "${GREEN}[+] Wallpaper symlink updated to: $WALLPAPER_STORE/$WALLPAPER_NAME${RESET}"

# Monitor Detection
MONITOR=$(hyprctl monitors | grep "Monitor" | awk '{print $2}' | head -n 1)
[ -z "$MONITOR" ] && MONITOR="eDP-1"

AUTOSTART_LINE="exec-once = mpvpaper -o \"--loop-file=inf --no-audio --hwdec=auto\" $MONITOR $SYMLINK_PATH"

# Clean old autostart lines to prevent duplicates, then append the fresh one
if [ -f "$HYPR_CONFIG" ]; then
    sed -i '/mpvpaper/d' "$HYPR_CONFIG"
    sed -i '/# Dynamic Live Wallpaper Manager/d' "$HYPR_CONFIG"
    
    echo -e "${YELLOW}[*] Updating autostart configuration in hyprland.conf...${RESET}"
    echo -e "\n# Dynamic Live Wallpaper Manager" >> "$HYPR_CONFIG"
    echo "$AUTOSTART_LINE" >> "$HYPR_CONFIG"
    echo -e "${GREEN}[+] Hyprland configuration updated successfully!${RESET}"
fi

# Apply live changes instantly
if pgrep mpvpaper > /dev/null; then
    echo -e "${YELLOW}[*] Reloading mpvpaper...${RESET}"
    killall mpvpaper
    sleep 0.5
fi

mpvpaper -o "--loop-file=inf --no-audio --hwdec=auto" "$MONITOR" "$SYMLINK_PATH" & disown

# Strip color codes for final clean output message
CLEAN_TITLE=$(echo -e "${DISPLAY_NAMES[$SELECTED_INDEX]}" | sed -r "s/\x1B\[([0-9]{1,3}(;[0-9]{1,2})?)?[mGK]//g")
echo -e "${GREEN}[✔] Wallpaper successfully set to: $CLEAN_TITLE${RESET}"