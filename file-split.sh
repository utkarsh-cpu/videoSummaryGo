#!/bin/bash

# Loop through all UFSFF Lecture files
for file in MM*_output.txt MM*_audio_output.txt MM*_video_output.txt; do
    if [ -f "$file" ]; then
        # Extract the base name (without the suffix)
        if [[ $file == *"_audio_output.txt" ]]; then
            folder_name="${file%_audio_output.txt}"
        elif [[ $file == *"_video_output.txt" ]]; then
            folder_name="${file%_video_output.txt}"
        else
            folder_name="${file%_output.txt}"
        fi

        # Create folder if it doesn't exist
        mkdir -p "$folder_name"

        # Move file to the folder
        mv "$file" "$folder_name/"

        echo "Moved $file to $folder_name/"
    fi
done