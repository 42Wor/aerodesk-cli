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
SYMLINK_PATH="$WALLPAPER_STORE/current_wallpaper" # No extension needed, mpv reads file headers
mkdir -p "$WALLPAPER_STORE"

DIRS=()
DISPLAY_NAMES=()
PLAYABLE=()
VIDEO_PATHS=()

# 1. Scan directories for ANY animated format
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
            # Show what format it is in the menu
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

# 2. Present options
echo -e "${BLUE}Available Wallpapers:${RESET}"
for i in "${!DIRS[@]}"; do
    echo -e "  [${GREEN}$((i+1))${RESET}] ${DISPLAY_NAMES[$i]}"
done

echo -ne "\n${YELLOW}Select a wallpaper to set (1-${#DIRS[@]}): ${RESET}"
read -r CHOICE

if ! [[ "$CHOICE" =~ ^[0-9]+$ ]] || [ "$CHOICE" -lt 1 ] || [ "$CHOICE" -gt "${#DIRS[@]}" ]; then
    echo -e "${RED}[!] Invalid choice.${RESET}"
    exit 1
fi

SELECTED_INDEX=$((CHOICE-1))

if [ "${PLAYABLE[$SELECTED_INDEX]}" -eq 0 ]; then
    echo -e "${RED}[!] Error: Folder '${DIRS[$SELECTED_INDEX]}' does not contain a playable video or GIF.${RESET}"
    exit 1
fi

# 3. Copy file and update symlink
VIDEO_PATH="${VIDEO_PATHS[$SELECTED_INDEX]}"
WALLPAPER_NAME=$(basename "$VIDEO_PATH")

cp "$VIDEO_PATH" "$WALLPAPER_STORE/$WALLPAPER_NAME"
ln -sf "$WALLPAPER_STORE/$WALLPAPER_NAME" "$SYMLINK_PATH"

echo -e "${GREEN}[+] Wallpaper symlink updated to: $WALLPAPER_STORE/$WALLPAPER_NAME${RESET}"

# 4. Monitor Detection & Hyprland config check
MONITOR=$(hyprctl monitors | grep "Monitor" | awk '{print $2}' | head -n 1)
[ -z "$MONITOR" ] && MONITOR="eDP-1"

HYPR_CONFIG="$HOME/.config/hypr/hyprland.conf"
AUTOSTART_LINE="exec-once = mpvpaper -o \"--loop-file=inf --no-audio --hwdec=auto\" $MONITOR $SYMLINK_PATH"

if [ -f "$HYPR_CONFIG" ]; then
    if grep -q "current_wallpaper" "$HYPR_CONFIG"; then
        echo -e "${GREEN}[+] Hyprland is already configured to use the dynamic symlink.${RESET}"
    else
        # Clean up old .mp4 specific symlink if it exists from previous script versions
        sed -i 's/current_wallpaper.mp4/current_wallpaper/g' "$HYPR_CONFIG" 2>/dev/null || true
        
        if ! grep -q "current_wallpaper" "$HYPR_CONFIG"; then
            echo -e "${YELLOW}[*] Appending live wallpaper setup to hyprland.conf...${RESET}"
            echo -e "\n# Dynamic Live Wallpaper Manager" >> "$HYPR_CONFIG"
            echo "$AUTOSTART_LINE" >> "$HYPR_CONFIG"
        fi
    fi
fi

# 5. Apply live changes instantly
if pgrep mpvpaper > /dev/null; then
    echo -e "${YELLOW}[*] Reloading mpvpaper...${RESET}"
    killall mpvpaper
    sleep 0.5
fi

mpvpaper -o "--loop-file=inf --no-audio --hwdec=auto" "$MONITOR" "$SYMLINK_PATH" & disown

# Strip color codes for final clean output message
CLEAN_TITLE=$(echo -e "${DISPLAY_NAMES[$SELECTED_INDEX]}" | sed -r "s/\x1B\[([0-9]{1,3}(;[0-9]{1,2})?)?[mGK]//g")
echo -e "${GREEN}[✔] Wallpaper successfully set to: $CLEAN_TITLE${RESET}"