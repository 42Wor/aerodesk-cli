#!/usr/bin/env bash

set -euo pipefail
shopt -s nullglob # Prevents loop errors if no files are found

# ANSI Color Codes
GREEN="\e[32m"
YELLOW="\e[33m"
BLUE="\e[34m"
RED="\e[31m"
RESET="\e[0m"

ARCHIVE_DIR="zip_folders"
mkdir -p "$ARCHIVE_DIR"

echo -e "${BLUE}==================================================${RESET}"
echo -e "${BLUE}        Smart Archive Extractor (unzip.sh)        ${RESET}"
echo -e "${BLUE}==================================================${RESET}"

# Check if directory is empty
if [ -z "$(ls -A "$ARCHIVE_DIR")" ]; then
    echo -e "${YELLOW}[*] The '$ARCHIVE_DIR' folder is empty.${RESET}"
    echo -e "    Please drop your downloaded .zip or .rar files into the '${BLUE}$ARCHIVE_DIR${RESET}' folder and run this script again."
    exit 0
fi

# Function to find the next available numbered folder (1, 2, 3...)
get_next_folder_number() {
    local max=0
    for d in */; do
        d="${d%/}"
        if [[ "$d" =~ ^[0-9]+$ ]]; then
            if (( d > max )); then
                max=$d
            fi
        fi
    done
    echo $((max + 1))
}

# Process RAR files
for file in "$ARCHIVE_DIR"/*.rar; do
    echo -e "${YELLOW}[*] Scanning RAR: $(basename "$file")...${RESET}"
    
    # Check if archive contains mp4, gif, webm, or mkv
    if unrar l "$file" | grep -iE '\.(mp4|gif|webm|mkv)$' > /dev/null; then
        NEXT_NUM=$(get_next_folder_number)
        mkdir -p "$NEXT_NUM"
        
        echo -e "    ${GREEN}[+] Live wallpaper found! Extracting to folder '$NEXT_NUM/'...${RESET}"
        unrar x -o+ "$file" "$NEXT_NUM/" > /dev/null
        
        rm -f "$file"
        echo -e "    ${GREEN}[+] Deleted original archive.${RESET}"
    else
        echo -e "    ${RED}[-] No live wallpaper files found. Skipping and keeping archive.${RESET}"
    fi
done

# Process ZIP files
for file in "$ARCHIVE_DIR"/*.zip; do
    echo -e "${YELLOW}[*] Scanning ZIP: $(basename "$file")...${RESET}"
    
    if unzip -l "$file" | grep -iE '\.(mp4|gif|webm|mkv)$' > /dev/null; then
        NEXT_NUM=$(get_next_folder_number)
        mkdir -p "$NEXT_NUM"
        
        echo -e "    ${GREEN}[+] Live wallpaper found! Extracting to folder '$NEXT_NUM/'...${RESET}"
        unzip -o "$file" -d "$NEXT_NUM/" > /dev/null
        
        rm -f "$file"
        echo -e "    ${GREEN}[+] Deleted original archive.${RESET}"
    else
        echo -e "    ${RED}[-] No live wallpaper files found. Skipping and keeping archive.${RESET}"
    fi
done

echo -e "\n${GREEN}[✔] Smart extraction complete! Run ./setup.sh to choose your wallpaper.${RESET}"