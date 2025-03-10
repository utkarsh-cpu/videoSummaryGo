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

## Version History

- first commit: basic code
- second commit: added tesseract support incase LLM is rejecting your video file input



## Prerequisites

Before using VideoSummaryGo, ensure you have the following installed or have:

- Go (version 1.15 or higher recommended)
- FFmpeg (for audio and video processing)
- Whisper.cpp (for audio transcription)
- tesseract-ocr (for backup video transcription)
- Google Gemini API Key (for main video transcription)
- Dependencies listed in `go.mod`


### Cloning this repository

- Repository with all its sub modules
```bash
git clone --recursive https://github.com/utkarsh-cpu/videoSummaryGo.git

```
- Only Repository
 ```bash
git clone  https://github.com/utkarsh-cpu/videoSummaryGo.git

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

### Installing tesseract

**Downloads and Installation Instructions**

**1. Windows**

*   **Recommended Installer (Easiest):** The easiest way to install Tesseract on Windows is to use a pre-built installer.  The most reliable and up-to-date installers are usually maintained by the community. Here's the preferred method:

    *   **UB-Mannheim Installer:**
        1.  **Go to the UB-Mannheim Tesseract page:** [https://github.com/UB-Mannheim/tesseract/wiki](https://github.com/UB-Mannheim/tesseract/wiki)
        2.  **Download the Installer:** Look for a link to the latest `.exe` installer file (e.g., `tesseract-ocr-w64-setup-v5.x.x.xxxxxxxx.exe`).  Choose the 64-bit version (w64) if you have a 64-bit system (most modern systems are).
        3.  **Run the Installer:** Double-click the downloaded `.exe` file.
        4.  **Important Installation Steps:**
            *   **Choose Components:**  During installation, make sure to select the language data you need.  "English" is usually selected by default.  You can select additional languages in the "Additional language data" section.  It's *much* easier to install language data during the initial installation than to add it later.
            *   **Add to PATH (Crucial):**  Make sure the installer adds Tesseract to your system's PATH environment variable.  There's usually a checkbox for this.  This allows you to run Tesseract from any command prompt or terminal window. If the installer doesn't do this automatically, you'll need to do it manually (see instructions below).
        5. **Verify Installation:**  Open a command prompt (search for "cmd" in the Start Menu) and type:  `tesseract --version`.  You should see the Tesseract version information.  If you get an error like "'tesseract' is not recognized...", the PATH wasn't set correctly.

    *   **Manually Adding to PATH (if needed):**
        1.  **Find the Tesseract Installation Directory:** This is usually something like `C:\Program Files\Tesseract-OCR`.
        2.  **Open System Properties:**  Search for "environment variables" in the Start Menu and select "Edit the system environment variables."
        3.  **Edit the PATH Variable:** In the "System Properties" window, click "Environment Variables...".  In the "System variables" section, find the "Path" variable, select it, and click "Edit...".
        4.  **Add the Tesseract Directory:**  Click "New" and add the full path to the Tesseract installation directory (e.g., `C:\Program Files\Tesseract-OCR`).  Click "OK" on all the windows to save the changes.  You might need to restart your computer or open a new command prompt for the changes to take effect.

* **Chocolatey (Package Manager - For advanced users):**
    If you use Chocolatey, you can install with:
    ```
    choco install tesseract
    ```
    You will still need to ensure language data is installed. You can install individual language packs (e.g., `choco install tesseract-lang-eng` for English).

**2. macOS**

*   **Homebrew (Recommended):** Homebrew is the most convenient package manager for macOS.

    1.  **Install Homebrew (if you don't have it):** Open Terminal and paste this command:
        ```bash
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        ```
        Follow the on-screen instructions.

    2.  **Install Tesseract:**
        ```bash
        brew install tesseract
        ```

    3.  **Install Language Data:** Homebrew installs English by default. To install other languages, use `brew install tesseract-lang`:
        ```bash
        brew install tesseract-lang
        ```
        This command will install ALL available languages. It can take up a lot of space. To install *specific* languages, find the language code (e.g., `spa` for Spanish, `fra` for French, `deu` for German) from the Tesseract documentation or by searching online, and then use:
        ```bash
        brew install tesseract-lang-spa  # For Spanish
        brew install tesseract-lang-fra  # For French
        brew install tesseract-lang-deu  # For German
        ```
        You can install multiple languages at once:
        ```bash
        brew install tesseract-lang-spa tesseract-lang-fra
        ```

    4. **Verify Installation:**
        ```bash
        tesseract --version
        ```

*   **MacPorts (Alternative Package Manager):**  If you prefer MacPorts:

    ```bash
    sudo port install tesseract
    sudo port install tesseract-<langcode>  # e.g., tesseract-eng, tesseract-spa
    ```

**3. Linux (Various Distributions)**

Linux distributions usually have Tesseract in their package repositories.  The specific command depends on your distribution:

*   **Debian/Ubuntu (and derivatives like Linux Mint, Pop!_OS):**

    ```bash
    sudo apt update
    sudo apt install tesseract-ocr
    sudo apt install tesseract-ocr-eng  # For English (usually installed by default)
    sudo apt install tesseract-ocr-spa  # For Spanish
    sudo apt install tesseract-ocr-<langcode> # For other languages
    ```

*   **Fedora/Red Hat/CentOS:**

    ```bash
    sudo dnf install tesseract
    sudo dnf install tesseract-langpack-eng  # For English
    sudo dnf install tesseract-langpack-spa  # For Spanish
    sudo dnf install tesseract-langpack-<langcode> # For other languages
    ```

*   **Arch Linux (and derivatives like Manjaro):**

    ```bash
    sudo pacman -S tesseract
    sudo pacman -S tesseract-data-eng  # For English
    sudo pacman -S tesseract-data-spa  # For Spanish
    sudo pacman -S tesseract-data-<langcode> # For other languages
    ```

*   **openSUSE:**

    ```bash
    sudo zypper install tesseract
    sudo zypper install tesseract-ocr-traineddata-english # For English
    sudo zypper install tesseract-ocr-traineddata-spanish # For Spanish
    sudo zypper install tesseract-ocr-traineddata-<langcode> # See note below
    ```
    *Note*:  For openSUSE, the naming convention for language packs might be slightly different. You can search for available language packs using `zypper search tesseract-ocr-traineddata`.

*   **Alpine Linux:**
    ```bash
    apk add tesseract tesseract-ocr-data-eng
    ```
    Replace `eng` with your required language code.

**4. Using Tesseract from the Command Line (Basic Example)**

Once you have Tesseract installed and the language data you need, you can use it from the command line like this:

```bash
tesseract imagename.png output.txt -l eng
```

*   `imagename.png`:  Replace this with the path to your image file (e.g., `images/my_scan.jpg`).  Tesseract supports various image formats (PNG, JPEG, TIFF, etc.).
*   `output.txt`:  This is the name of the text file where Tesseract will write the extracted text.
*   `-l eng`:  This specifies the language (English in this case).  Use the appropriate language code (e.g., `-l spa` for Spanish, `-l fra` for French).  You can specify multiple languages by separating them with a plus sign (e.g., `-l eng+spa`).


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
/path/to/whisper.cpp/build/bin/whisper-cli -h
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
