#!/bin/bash
# UFSFF Folder to PDF Converter
# Created: March 10, 2025
# This script finds all UFSFF lecture folders, converts txt files to md, and then to pdf

# Create output directory for PDFs
OUTPUT_DIR="PDFs_UFSFF"
mkdir -p "$OUTPUT_DIR"

echo "Starting UFSFF conversion process..."

# Use proper quoting to handle spaces in folder names
UFSFF_FOLDERS=$(find . -type d -name "UFSFF*" -not -path "*/$OUTPUT_DIR/*" | sort)

# Check if any UFSFF folders were found
if [ -z "$UFSFF_FOLDERS" ]; then
    echo "No UFSFF folders found."
    exit 1
fi

# Count folders properly with wc -l
FOLDER_COUNT=$(echo "$UFSFF_FOLDERS" | wc -l)
echo "Found $FOLDER_COUNT UFSFF folders."

# Properly count total text files, preserving spaces in folder names
TOTAL_FILES=0
while IFS= read -r folder; do
    # Count files with proper quoting
    file_count=$(find "$folder" -name "*.txt" | wc -l)
    TOTAL_FILES=$((TOTAL_FILES + file_count))
done <<< "$UFSFF_FOLDERS"

echo "Total files to process: $TOTAL_FILES"

# Check if we should use the existing md_to_pdf.sh script
if [ -f "./md_to_pdf.sh" ]; then
    echo "Found md_to_pdf.sh script, will use it for PDF conversion."
    chmod +x ./md_to_pdf.sh
    USE_CUSTOM_SCRIPT=true
else
    echo "md_to_pdf.sh not found. Will use pandoc for PDF conversion."
    USE_CUSTOM_SCRIPT=false

    # Check if pandoc is installed
    if ! command -v pandoc &> /dev/null; then
        echo "Error: pandoc is not installed. Please install pandoc or provide md_to_pdf.sh."
        exit 1
    fi
fi

# Counter for progress tracking
COUNT=0

# Process each folder - properly preserve spaces in folder names
while IFS= read -r folder; do
    folder_name=$(basename "$folder")
    echo "Processing folder: $folder_name"

    # Create folder structure in output directory
    mkdir -p "$OUTPUT_DIR/$folder_name"

    # Find all text files in the folder - with proper quoting
    txt_files=$(find "$folder" -name "*.txt")

    # Process each text file - with proper quoting
    while IFS= read -r txt_file; do
        if [[ "$file" == *"_audio_output.txt" || "$file" == *"_video_output.txt" ]]; then
              echo "Skipping $file (audio/video file)"
              continue
        fi
        # Skip if txt_file is empty
        [ -z "$txt_file" ] && continue

        # Increment counter
        ((COUNT++))

        # Calculate progress percentage
        PROGRESS=$((COUNT * 100 / (TOTAL_FILES/3)))

        # Get the file name without extension
        file_name=$(basename "$txt_file" .txt)

        echo "[$PROGRESS%] Converting: $file_name"

        # Create MD file path
        md_file="$OUTPUT_DIR/$folder_name/$file_name.md"
        pdf_file="$OUTPUT_DIR/$folder_name/$file_name.pdf"

        # Copy TXT to MD
        cp "$txt_file" "$md_file"

        # Convert MD to PDF
        if [ "$USE_CUSTOM_SCRIPT" = true ]; then
            # Use existing md_to_pdf.sh script
            ./md_to_pdf.sh "$md_file"
        else
            # Use pandoc
            pandoc "$md_file" -o "$pdf_file"
        fi

        echo "  Created: $(basename "$pdf_file")"
    done <<< "$txt_files"
done <<< "$UFSFF_FOLDERS"

echo "✅ Conversion complete! All PDFs are in the $OUTPUT_DIR directory."
