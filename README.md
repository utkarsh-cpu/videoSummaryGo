# VideoSummaryGo

A Go-based tool that automatically generates summaries from video content by processing audio transcriptions and analyzing video frames.

## Overview

VideoSummaryGo extracts valuable information from videos by transcribing audio and analyzing visual content. The application processes this information to create comprehensive text summaries that can be converted to PDF format for easy sharing and reading.

## Features

- Audio extraction and transcription from video files
- Video frame analysis for relevant content
- Text-based summary generation
- PDF conversion support
- Handling of large files through splitting

## Prerequisites

Before using VideoSummaryGo, ensure you have the following installed:

- Go (version 1.15 or higher recommended)
- FFmpeg (for audio and video processing)
- Whisper.cpp (for audio transcription)
- Dependencies listed in `go.mod`


### Cloning this repository

- Repository with all its sub modules
```bash
git clone --recursive https://github.com/utkarsh-cpu/videoSummaryGo.git

```
- Only Repository
 ```bash
git clone --recursive https://github.com/utkarsh-cpu/videoSummaryGo.git

```

### Installing FFmpeg

FFmpeg is required for extracting audio from videos and processing media files. Follow the steps below to install it:

#### For Windows:

1. Download FFmpeg:
   - Visit the [official FFmpeg website](https://ffmpeg.org/download.html)
   - Navigate to Windows builds (gyan.dev or BtbN are recommended options)
   - Download the full build package (ffmpeg-git-full.7z)[6]

2. Extract the files:
   - Use 7-Zip or another archive utility to extract the downloaded file[2]
   - Rename the extracted folder to "FFmpeg" for simplicity[6]
   - Move this folder to the root of your C: drive (creating `C:\FFmpeg`)[6]

3. Add FFmpeg to PATH:
   - Search for "Edit the system environment variables" in Windows search
   - Click "Environment Variables" in the System Properties window
   - Under "System Variables," find and select "Path", then click "Edit"
   - Click "New" and add `C:\FFmpeg\bin`[6][2]
   - Click "OK" to close all dialog boxes

4. Verify the installation:
   - Open Command Prompt and type `ffmpeg`
   - If installed correctly, you should see version information and configuration details[6][8]

#### For Linux:

Use your distribution's package manager to install FFmpeg:
```bash
# For Ubuntu/Debian
sudo apt update
sudo apt install ffmpeg

# For CentOS/RHEL
sudo dnf install ffmpeg
```

#### For macOS:

Install using Homebrew:
```bash
brew install ffmpeg
```

### Building Whisper.cpp with CMake

Whisper.cpp is needed for audio transcription. Here's how to build it:

1. Clone the whisper.cpp repository:
   ```bash
   ## if submodeules are not cloned
   git clone https://github.com/ggerganov/whisper.cpp
   cd whisper.cpp
   
   ## else 
   cd whisper.cpp
   ```

2. Configure and build with CMake:
   ```bash
   ## if you dont have gpu
   cmake -B build
   cmake --build build --config Release
   ## if you have nvidia gpu
   cmake -B build -DGGML_CUDA=1
   cmake --build build --config Release
   ```

3. Download a Whisper model:
   ```bash
   # Return to the main directory
   cd ..
   
   # Make the download script executable
   chmod +x models/download-ggml-model.sh
   
   # Download a model (e.g., tiny.en, base.en, small.en, medium.en, large-v3)
   ./models/download-ggml-model.sh medium.en
   ```

4. Integrate with VideoSummaryGo:
   - Copy the compiled whisper binary and model to a location accessible by VideoSummaryGo
   - Update the configuration to point to these files

### Verifying Prerequisites

Before running VideoSummaryGo, verify all prerequisites are installed correctly:

```bash
# Check Go version
go version

# Check FFmpeg installation
ffmpeg -version

# Test whisper.cpp installation
/path/to/whisper/build/bin/whisper-cli -h
```

All components must be properly installed and accessible for VideoSummaryGo to function correctly.


## How to Use

### Basic Usage

1. Run the main application with your video file:
   ```
   go build main.go
   ./main gemini-pro YOUR_API_KEY 60 ./whisper-cpp/build/bin/whisper-cli ./whisper-cpp/models/ggml-medium.en.bin 4 en ./videos/lecture.mp4
   ```

2. The application will:
   - Extract audio from the video
   - Transcribe the audio content
   - Analyze key video frames if applicable
   - Generate a summary in text format

3. Find your summary files in the directory

### Using Utility Scripts

#### Make folder for various txt files

Since we will have three output.txt for a single file, we will concat in single folder:

```
./file-split.sh [input_video.mp4] [chunk_size_in_MB]
```

#### Convert Text Summary to PDF

Convert your generated summary to PDF format:

```
./txt_to_pdf.sh [input_summary.txt] [output_file.pdf]
```

## Project Structure

- `audio_transcript/`: Contains components for audio processing and transcription
- `video_image_transcription/`: Handles video frame extraction and analysis
- `main.go`: Main application entry point
- `/whisper.cpp` : Whisper.cpp folder
- `file-split.sh`: Utility for splitting large files
- `txt_to_pdf.sh`: Converts text summaries to PDF format

## Troubleshooting

- **ffmpeg errors**: Ensure ffmpeg is correctly installed and in your system PATH
- **gemini**: Ensure you have api-key for gemini, to get your check https://aistudio.google.com
- **Transcription accuracy problems**: Try adjusting language settings if available

## Contributing

Contributions to VideoSummaryGo are welcome! Please feel free to submit issues or pull requests to improve the tool.
