package videoSummaryGo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ChunkData struct
type ChunkData struct {
	VideoPath  string
	AudioPath  string
	ChunkNum   int
	Err        error
	VideoIndex int
	BaseName   string
}

func YoutubeDownloader(url string, customDestDir string) (string, error) {
	// Validate dependencies and URL
	ytDlpPath, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", fmt.Errorf("yt-dlp not found in PATH: %w", err)
	}

	if !isValidYoutubeURL(url) {
		return "", fmt.Errorf("invalid YouTube URL: %s", url)
	}

	// Setup directories
	tempDir, err := os.MkdirTemp("", "youtube_download_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	destDir := getDestinationDir(customDestDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Download video
	outputTemplate := filepath.Join(tempDir, "%(title)s-%(id)s.%(ext)s")
	stdout, stderr, err := executeYTDLP(ytDlpPath, url, outputTemplate)
	if err != nil {
		return "", fmt.Errorf("download failed: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Process downloaded file
	tempFilePath, err := findDownloadedFile(stdout, tempDir)
	if err != nil {
		return "", err
	}

	// Move to destination
	return moveToDestination(tempFilePath, destDir)
}

func isValidYoutubeURL(url string) bool {
	validPrefixes := []string{
		"https://www.youtube.com/",
		"https://youtu.be/",
		"https://youtube.com/",
	}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}

func getDestinationDir(customDir string) string {
	var destDir string
	if customDir != "" {
		destDir = customDir
	} else {
		// Default to Videos directory in the current working directory
		cwd, err := os.Getwd()
		if err != nil {
			// Fallback: Use a relative path if Getwd fails, though this is unlikely
			log.Printf("Warning: Failed to get current directory: %v. Using relative path 'Videos'.", err)
			return "Videos"
		}
		destDir = filepath.Join(cwd, "Videos")
	}

	// Ensure the path is absolute
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		log.Printf("Warning: Failed to convert destination directory '%s' to absolute path: %v. Using original.", destDir, err)
		return destDir // Return original if Abs fails
	}

	log.Printf("Resolved Destination Directory (Absolute): %s\n", absDestDir)
	return absDestDir
}

func executeYTDLP(ytDlpPath, url, outputTemplate string) (string, string, error) {
	var stdout, stderr strings.Builder
	cmd := exec.Command(ytDlpPath,
		"-o", outputTemplate,
		"--merge-output-format", "mp4",
		"--no-mtime",
		"--retries", "3",
		"--fragment-retries", "10",
		"--force-ipv4",
		url,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func moveToDestination(tempFilePath, destDir string) (string, error) {
	fileName := sanitizeFilename(filepath.Base(tempFilePath))

	// Ensure destDir is absolute (should already be, but double-check)
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for destination directory '%s': %w", destDir, err)
	}

	destPath := filepath.Join(absDestDir, fileName)

	// Ensure destPath is absolute (it should be if absDestDir and fileName are correct)
	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for final destination '%s': %w", destPath, err)
	}

	fmt.Printf("Attempting to move temp file '%s' to '%s'\n", tempFilePath, absDestPath)

	// Remove existing file if present
	if _, err := os.Stat(absDestPath); err == nil {
		log.Printf("Removing existing file at destination: %s\n", absDestPath)
		if err := os.Remove(absDestPath); err != nil {
			// Log warning but proceed, copy might overwrite
			log.Printf("Warning: Failed to remove existing file at '%s': %v", absDestPath, err)
		}
	}

	// Copy file
	if err := copyFile(tempFilePath, absDestPath); err != nil {
		return "", fmt.Errorf("failed to copy file from '%s' to '%s': %w", tempFilePath, absDestPath, err)
	}

	log.Printf("Successfully copied file to: %s\n", absDestPath)
	return absDestPath, nil // Return the final absolute path
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func sanitizeFilename(filename string) string {
	replacer := strings.NewReplacer(
		"?", "_", "？", "_", // Added full-width question mark
		"*", "_", ":", "_",
		"<", "_", ">", "_", "\"", "_",
		"|", "_", "/", "_", "\\", "_",
	)
	// Also replace consecutive underscores resulting from replacements
	sanitized := replacer.Replace(filename)
	re := regexp.MustCompile(`_+`) // Match one or more underscores
	sanitized = re.ReplaceAllString(sanitized, "_")
	// Trim leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")
	// Optional: Handle potential length issues (though less common now)
	// maxLen := 200 // Example limit
	// if len(sanitized) > maxLen {
	//  sanitized = sanitized[:maxLen]
	// }
	return sanitized
}

func extractFilenameFromOutput(output string, destinationDir string) (string, error) {
	// Prioritize the message indicating the final merged/processed file
	// Regex updated to be less greedy and handle quotes possibly within filename (though unlikely)
	mergePattern := `\[(?:ffmpeg|Merger|ExtractAudio|ModifyChapters|FixupM3u8|FixupTimestamp|FixupDuration)\] Merging formats into "([^"]+)"`
	if match := regexp.MustCompile(mergePattern).FindStringSubmatch(output); len(match) > 1 {
		potentialPath := match[1]
		// This path from the log is often relative to the CWD yt-dlp ran in (which *is* the tempDir because of -o)
		// Or sometimes it's absolute. Best bet: join with tempDir and check.
		fullPath := filepath.Join(destinationDir, filepath.Base(potentialPath)) // Use Base to be safe
		if _, err := os.Stat(fullPath); err == nil {
			log.Printf("Extracted final filename from merge log: %s", fullPath)
			return fullPath, nil
		}
		// If not found when joined, maybe the log path was already absolute? Check that.
		if filepath.IsAbs(potentialPath) {
			if _, err := os.Stat(potentialPath); err == nil {
				log.Printf("Extracted final filename (absolute) from merge log: %s", potentialPath)
				return potentialPath, nil
			}
		}
		log.Printf("Warning: Merge pattern matched '%s', but couldn't verify path '%s' or '%s'.", potentialPath, fullPath, potentialPath)
		// Fall through to Destination pattern
	}

	// Fallback to the initial destination message - This might be an intermediate file!
	destPattern := `\[download\] Destination: (.*)`
	if match := regexp.MustCompile(destPattern).FindStringSubmatch(output); len(match) > 1 {
		filePath := match[1]
		// This path *should* be the one specified by -o, hence within destinationDir
		log.Printf("Extracted filename from destination log: %s", filePath)
		if _, err := os.Stat(filePath); err == nil {
			// Return this only as a fallback, maybe log a warning
			log.Printf("Warning: Using filename from 'Destination:' log, might be intermediate: %s", filePath)
			return filePath, nil
		}
		log.Printf("Warning: Destination pattern matched '%s', but os.Stat failed.", filePath)
	}

	return "", fmt.Errorf("filename not found reliably in yt-dlp output")
}

// Adjust findDownloadedFile to be slightly more robust with fallbacks
func findDownloadedFile(stdout, tempDir string) (string, error) {
	log.Printf("Attempting to extract filename from yt-dlp stdout...")
	if path, err := extractFilenameFromOutput(stdout, tempDir); err == nil {
		log.Printf("Found path via regex: %s", path)
		// Double-check existence here before returning
		if _, statErr := os.Stat(path); statErr == nil {
			// Check if it's the FINAL expected format (mp4)
			if strings.HasSuffix(strings.ToLower(path), ".mp4") && !regexp.MustCompile(`\.f\d+\.mp4$`).MatchString(path) {
				log.Printf("Regex found final MP4 path: %s", path)
				return path, nil
			} else {
				log.Printf("Warning: Regex found path '%s', but it looks like an intermediate format or non-MP4. Will try Glob.", path)
				// Continue to Glob fallback
			}

		} else {
			log.Printf("Warning: Regex found path '%s', but os.Stat failed: %v. Falling back to glob.", path, statErr)
			// Continue to Glob fallback
		}
	} else {
		log.Printf("Failed to extract filename via regex: %v. Falling back to glob.", err)
	}

	// Fallback: find the final MP4 file specifically
	log.Printf("Falling back to searching for *.mp4 in %s", tempDir)
	globPattern := filepath.Join(tempDir, "*.mp4")
	files, err := filepath.Glob(globPattern)
	if err != nil {
		return "", fmt.Errorf("error during glob search '%s': %w", globPattern, err)
	}

	// Filter out intermediate files if multiple MP4s are found
	finalFiles := []string{}
	intermediateRegex := regexp.MustCompile(`\.f\d+\.mp4$`)
	for _, file := range files {
		if !intermediateRegex.MatchString(file) {
			finalFiles = append(finalFiles, file)
		}
	}

	if len(finalFiles) == 1 {
		log.Printf("Found final MP4 file via glob: %s", finalFiles[0])
		return finalFiles[0], nil
	} else if len(finalFiles) > 1 {
		log.Printf("Warning: Found multiple potential final MP4 files via glob: %v. Returning the first one: %s", finalFiles, finalFiles[0])
		return finalFiles[0], nil // Or return error? Choosing first is pragmatic.
	} else {
		// If no non-intermediate MP4, maybe merge failed? Look for *any* MP4 as last resort.
		if len(files) > 0 {
			log.Printf("Warning: No clear final MP4 found via glob. Returning first MP4 found (might be intermediate): %s", files[0])
			return files[0], nil
		}
		// Last resort: Check other common extensions
		otherVideoPatterns := []string{"*.mkv", "*.webm", "*.mov", "*.avi"}
		for _, pattern := range otherVideoPatterns {
			otherFiles, _ := filepath.Glob(filepath.Join(tempDir, pattern))
			if len(otherFiles) > 0 {
				log.Printf("Found fallback video file (non-mp4): %s", otherFiles[0])
				return otherFiles[0], nil
			}
		}
		return "", fmt.Errorf("no suitable video file found in temp directory '%s' via glob", tempDir)
	}
}

// SetLlmApi function
func SetLlmApi(llm string, apiKey string) (*genai.Client, *genai.GenerativeModel, context.Context) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
	}
	model := client.GenerativeModel(llm)
	fmt.Println("LLM API setup complete.")
	return client, model, ctx
}

const (
	maxRetries = 5                // Maximum number of retry attempts for LLM calls
	retryDelay = 30 * time.Second // Delay between retry attempts
)

// sentLlmPrompt function
func sentLlmPrompt(model *genai.GenerativeModel, prompt []genai.Part, ctx context.Context, file *os.File, videoIndex int) string {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		fmt.Printf("Sending combined prompt for video %d to LLM, attempt %d...\n", videoIndex, attempt+1)
		startTime := time.Now()
		resp, err := model.GenerateContent(ctx, prompt...)
		if err == nil {
			duration := time.Since(startTime)
			fmt.Printf("LLM response received for video %d in %v.\n", videoIndex, duration)
			var llmResponse string
			for _, c := range resp.Candidates {
				if c.Content != nil {
					for _, part := range c.Content.Parts {
						if text, ok := part.(genai.Text); ok {
							llmResponse += string(text)
							if file != nil { // Check if file is nil before writing
								if _, err := fmt.Fprintln(file, string(text)); err != nil {
									log.Println("Error writing to file:", err)
								}
							}
						}
					}
				}
			}
			fmt.Printf("Combined prompt processed and written to file for video %d.\n", videoIndex)
			return llmResponse
		}

		log.Printf("Error generating content for video %d (attempt %d): %v\n", videoIndex, attempt+1, err)
		if attempt < maxRetries {
			fmt.Printf("Retrying in %v...\n", retryDelay)
			time.Sleep(retryDelay)
		} else {
			fmt.Printf("Max retries reached for video %d. Aborting LLM call.\n", videoIndex)
			return "" // Return empty string if max retries reached
		}
	}
	return "" // Should not reach here, but added for completeness
}

// chunkVideo function
func chunkVideo(videoPath string, chunkDuration int, videoIndex int, baseName string) ([]ChunkData, error) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "video_chunks")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory: %w", err)
	}

	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("error getting video duration: %w, output: %s", err, string(output))
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("error parsing video duration: %w", err)
	}

	numChunks := int(duration / float64(chunkDuration))
	if int(duration)%chunkDuration != 0 {
		numChunks++
	}

	var chunks []ChunkData

	for i := 0; i < numChunks; i++ {
		startTime := i * chunkDuration
		chunkVideoPath := fmt.Sprintf("%s/chunk_%d_video_%d.mp4", tempDir, i, videoIndex)
		chunkAudioPath := fmt.Sprintf("%s/chunk_%d_video_%d.wav", tempDir, i, videoIndex)

		cmd := exec.Command("ffmpeg",
			"-ss", fmt.Sprintf("%d", startTime),
			"-i", videoPath,
			"-t", fmt.Sprintf("%d", chunkDuration),
			"-c", "copy",
			"-an", chunkVideoPath,
			"-ss", fmt.Sprintf("%d", startTime),
			"-i", videoPath,
			"-t", fmt.Sprintf("%d", chunkDuration),
			"-vn",
			"-acodec", "pcm_s16le", // 16-bit WAV audio
			chunkAudioPath,
		)

		output, err = cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("error creating video chunk %d for video %d: %w, output: %s", i, videoIndex, err, string(output))
		}
		chunks = append(chunks, ChunkData{VideoPath: chunkVideoPath, AudioPath: chunkAudioPath, ChunkNum: i, VideoIndex: videoIndex, BaseName: baseName})
	}

	return chunks, nil
}

// TranscribeAudioWhisperCLI function
func TranscribeAudioWhisperCLI(audioPath string, whisperCLIPath string, whisperModelPath string, videoIndex int, chunkNum int, threads int, language string) (string, error) {
	cmdArgs := []string{
		"--model", whisperModelPath,
		"--threads", fmt.Sprintf("%d", threads),
	}
	if language != "" {
		cmdArgs = append(cmdArgs, "--language", language)
	}
	cmdArgs = append(cmdArgs, audioPath)

	cmd := exec.Command(whisperCLIPath, cmdArgs...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	fmt.Printf("Starting whisper-cli for video %d chunk %d, Audio Path: %s\n", videoIndex, chunkNum, audioPath)
	startTime := time.Now()

	err := cmd.Run()
	duration := time.Since(startTime)
	fmt.Printf("Whisper-cli finished for video %d chunk %d in %v\n", videoIndex, chunkNum, duration)

	if err != nil {
		return "", fmt.Errorf("error running whisper-cli for video %d chunk %d: %w, stderr: %s", videoIndex, chunkNum, err, stderr.String())
	}

	transcript := out.String()
	return transcript, nil
}

// extractFrames function
func extractFrames(videoPath string, videoIndex int, chunkNum int) ([]string, error) {
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("frames_video%d_chunk%d", videoIndex, chunkNum))
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory for frames: %w", err)
	}

	// Extract frames at 1fps.  Adjust -r as needed.
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-r", "1", // Frames per second
		"-q:v", "2", // JPEG quality (2 is high)
		fmt.Sprintf("%s/frame_%%04d.jpg", tempDir),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("error extracting frames: %w, output: %s", err, string(output))
	}

	// Get list of extracted frame files
	var framePaths []string
	filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".jpg") {
			framePaths = append(framePaths, path)
		}
		return nil
	})

	return framePaths, nil
}

// TranscribeVideoTesseractAPIAPI function
func TranscribeVideoTesseractAPI(framePaths []string) (string, error) {
	var combinedTranscript strings.Builder
	var wg sync.WaitGroup
	frameResults := make(chan struct {
		Text  string
		Error error
	}, len(framePaths)) // Buffered channel for results

	// Limit concurrency to the number of CPUs (or a reasonable limit)
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 { //  cap it to 8 for now to avoid too many subprocesses
		numWorkers = 8
	}
	guard := make(chan struct{}, numWorkers) // Semaphore

	for _, framePath := range framePaths {
		wg.Add(1)
		guard <- struct{}{} // Acquire a slot

		go func(fp string) {
			defer wg.Done()
			defer func() { <-guard }() // Release the slot

			// Open the image file
			imgFile, err := os.Open(fp)
			if err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error opening image file %s: %w", fp, err)}
				return
			}

			// Decode the image
			img, _, err := image.Decode(imgFile)
			imgFile.Close() // Close immediately after decoding
			if err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error decoding image file %s: %w", fp, err)}
				return
			}

			// Convert to JPEG
			buf := new(bytes.Buffer)
			if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error encoding image to JPEG: %w", err)}
				return
			}
			jpegBytes := buf.Bytes()

			tempFile, err := os.CreateTemp("", "ocr_*.jpg")
			if err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error creating temp file: %w", err)}
				return
			}
			tempFilePath := tempFile.Name()
			defer os.Remove(tempFilePath)

			_, err = tempFile.Write(jpegBytes)
			if err != nil {
				tempFile.Close() // Close before removing
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error writing to temp file: %w", err)}
				return
			}
			if err := tempFile.Close(); err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error closing temp file: %w", err)}
				return
			}

			cmd := exec.Command("tesseract", tempFilePath, "stdout")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err = cmd.Run()
			if err != nil {
				frameResults <- struct {
					Text  string
					Error error
				}{"", fmt.Errorf("error running tesseract on %s: %w, stderr: %s", fp, err, stderr.String())}
				return
			}

			frameResults <- struct {
				Text  string
				Error error
			}{stdout.String(), nil}

		}(framePath)
	}

	wg.Wait()           // Wait for all goroutines to finish
	close(frameResults) // Close the channel - no more results coming

	// Collect results from the channel
	for result := range frameResults {
		if result.Error != nil {
			log.Println(result.Error) // Log individual errors
			continue                  // Skip frames with errors
		}
		combinedTranscript.WriteString(result.Text)
		combinedTranscript.WriteString("\n")
	}

	return combinedTranscript.String(), nil
}

// transcribeVideoLLM function
func transcribeVideoLLM(ctx context.Context, client *genai.Client, model *genai.GenerativeModel, videoPath string, videoIndex int, chunkNum int) (string, error) {
	uploadedFile, err := client.UploadFileFromPath(ctx, videoPath, nil)
	if err != nil {
		// If LLM fails, fall back to Tesseract
		fmt.Printf("Chunk %d for video %d: LLM upload failed, falling back to Tesseract...\n", chunkNum, videoIndex)
		framePaths, err := extractFrames(videoPath, videoIndex, chunkNum)
		if err != nil {
			return "", fmt.Errorf("error extracting frames for video %d chunk %d: %w", videoIndex, chunkNum, err)
		}
		transcript, err := TranscribeVideoTesseractAPI(framePaths)
		if err != nil {
			return "", fmt.Errorf("error transcribing frames with Tesseract for video %d chunk %d: %w", videoIndex, chunkNum, err)
		}

		// Cleanup extracted frames.
		if len(framePaths) > 0 {
			os.RemoveAll(filepath.Dir(framePaths[0]))
		}
		return transcript, nil

	}

	fmt.Println("Waiting for 30 seconds after file upload to ensure file activation...")
	time.Sleep(60 * time.Second) // Wait for file to be ready
	defer func() { client.DeleteFile(ctx, uploadedFile.Name) }()

	fmt.Printf("Chunk %d for video %d: Video chunk uploaded as: %s\n", chunkNum, videoIndex, uploadedFile.URI)

	promptList := []genai.Part{
		genai.FileData{URI: uploadedFile.URI},
		genai.Text("## Task Description\nAnalyze the video and provide a detailed raw transcription of text displayed in the video."),
	}
	videoTranscript := sentLlmPrompt(model, promptList, ctx, nil, videoIndex) // No file writing here

	if videoTranscript == "" {
		// If LLM transcription fails, fall back to Tesseract
		fmt.Printf("Chunk %d for video %d: LLM transcription failed, falling back to Tesseract...\n", chunkNum, videoIndex)
		framePaths, err := extractFrames(videoPath, videoIndex, chunkNum)
		if err != nil {
			return "", fmt.Errorf("error extracting frames for video %d chunk %d: %w", videoIndex, chunkNum, err)
		}
		transcript, err := TranscribeVideoTesseractAPI(framePaths)
		// Cleanup extracted frames.
		if len(framePaths) > 0 {
			os.RemoveAll(filepath.Dir(framePaths[0]))
		}
		if err != nil {
			return "", fmt.Errorf("error transcribing frames with Tesseract for video %d chunk %d: %w", videoIndex, chunkNum, err)
		}
		return transcript, nil
	}

	fmt.Printf("Chunk %d for video %d: Video transcribed by LLM.\n", chunkNum, videoIndex)

	return videoTranscript, nil
}

// processChunk function
func processChunk(chunkData ChunkData, client *genai.Client, model *genai.GenerativeModel, ctx context.Context, errorChannel chan<- error, whisperCLIPath string, whisperModelPath string, whisperThreads int, whisperLanguage string, audioOutputFile, videoOutputFile *os.File) {
	chunk := chunkData

	if chunk.Err != nil {
		errorChannel <- chunk.Err
		return
	}

	fmt.Printf("Processing chunk %d for video %d...\n", chunk.ChunkNum, chunk.VideoIndex)
	defer fmt.Printf("Finished processing chunk %d for video %d.\n", chunk.ChunkNum, chunk.VideoIndex)

	var wg sync.WaitGroup
	wg.Add(2) // We have two goroutines: audio and video transcription

	var audioTranscript string
	var audioErr error
	go func() {
		defer wg.Done()
		audioTranscript, audioErr = TranscribeAudioWhisperCLI(chunk.AudioPath, whisperCLIPath, whisperModelPath, chunk.VideoIndex, chunk.ChunkNum, whisperThreads, whisperLanguage)
		if audioErr != nil {
			errorChannel <- fmt.Errorf("error transcribing audio for video %d chunk %d: %w", chunk.VideoIndex, chunk.ChunkNum, audioErr)
			audioTranscript = fmt.Sprintf("Audio transcription failed for video %d chunk %d.", chunk.VideoIndex, chunk.ChunkNum)
		}
		// Write to audio output file *immediately*
		_, err := fmt.Fprintf(audioOutputFile, "Video Index: %d, Chunk: %d\n%s\n", chunk.VideoIndex, chunk.ChunkNum, audioTranscript)
		if err != nil {
			errorChannel <- fmt.Errorf("error writing to audio file for video %d chunk %d: %v", chunk.VideoIndex, chunk.ChunkNum, err)
		}
		fmt.Printf("Chunk %d for video %d: Audio transcribed and written to audio output file.\n", chunk.ChunkNum, chunk.VideoIndex)
		os.Remove(chunk.AudioPath) // Delete audio chunk
	}()

	var videoTranscript string
	var videoErr error
	go func() {
		defer wg.Done()
		videoTranscript, videoErr = transcribeVideoLLM(ctx, client, model, chunk.VideoPath, chunk.VideoIndex, chunk.ChunkNum)
		if videoErr != nil {
			errorChannel <- fmt.Errorf("error transcribing video for video %d chunk %d: %w", chunk.VideoIndex, chunk.ChunkNum, videoErr)
			videoTranscript = fmt.Sprintf("Video transcription failed for video %d chunk %d.", chunk.VideoIndex, chunk.ChunkNum)
		}
		// Write to video output file *immediately*
		_, err := fmt.Fprintf(videoOutputFile, "Video Index: %d, Chunk: %d\n%s\n", chunk.VideoIndex, chunk.ChunkNum, videoTranscript)
		if err != nil {
			errorChannel <- fmt.Errorf("error writing to video file for video %d chunk %d: %v", chunk.VideoIndex, chunk.ChunkNum, err)
		}
		fmt.Printf("Chunk %d for video %d: Video transcribed and written to video output file.\n", chunk.ChunkNum, chunk.VideoIndex)
		os.Remove(chunk.VideoPath) // Delete video chunk
	}()

	wg.Wait() // Wait for both goroutines to complete

}

func VideoSummary(llm string, apiKey string, chunkDuration int, whisperCLIPath string, whisperModelPath string, whisperThreads int, whisperLanguage string, inputPath string, inputFromUser string) error {
	runtime.GOMAXPROCS(runtime.NumCPU())

	client, model, ctx := SetLlmApi(llm, apiKey)
	defer client.Close()

	errorChannel := make(chan error, 10) // Buffered channel

	var videoPaths []string
	// Ensure inputPath is absolute BEFORE stat check
	absInputPath, err := filepath.Abs(inputPath)
	if err != nil {
		// Use log.Fatalf as this is a critical starting point error
		log.Fatalf("Error converting input path '%s' to absolute: %v\n", inputPath, err)
	}

	fileInfo, err := os.Stat(absInputPath)
	if err != nil {
		// The stat check should now use the absolute path
		log.Fatalf("Error accessing input path '%s': %v\n", absInputPath, err)
	}

	// Use absInputPath consistently from now on
	inputPath = absInputPath // Update inputPath to the absolute version

	if fileInfo.IsDir() {
		fmt.Println("Processing folder:", inputPath)
		err = filepath.WalkDir(inputPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && IsVideoFile(path) {
				absVidPath, err := filepath.Abs(path) // Ensure stored path is absolute
				if err != nil {
					log.Printf("Warning: Could not get absolute path for %s: %v", path, err)
					videoPaths = append(videoPaths, path) // Add original as fallback
				} else {
					videoPaths = append(videoPaths, absVidPath)
				}
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error walking directory: %v\n", err)
		}
	} else {
		fmt.Println("Processing single file:", inputPath) // Already absolute
		if IsVideoFile(inputPath) {
			videoPaths = append(videoPaths, inputPath)
		} else {
			log.Printf("Warning: Input path '%s' is not a video file.\n", inputPath)
		}
	}

	if len(videoPaths) == 0 {
		fmt.Println("No video files found to process.")
		return nil
	}

	for videoIndex, videoPath := range videoPaths {
		// videoPath should now be absolute
		videoDir := filepath.Dir(videoPath) // Get the directory of the video
		baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

		// Create output files in the *same directory* as the video
		outputFileName := filepath.Join(videoDir, baseName+"_output.txt")
		audioOutputFileName := filepath.Join(videoDir, baseName+"_audio_output.txt")
		videoOutputFileName := filepath.Join(videoDir, baseName+"_video_output.txt")

		fmt.Printf("\n--- START PROCESSING VIDEO %d: %s ---\n", videoIndex+1, videoPath)
		fmt.Printf("Creating output files in directory: %s\n", videoDir)

		outputFile, err := os.Create(outputFileName)
		if err != nil {
			log.Fatalf("Error creating output file for video %s: %v\n", videoPath, err)
			continue // Continue to the next video
		}
		defer outputFile.Close()

		audioOutputFile, err := os.Create(audioOutputFileName)
		if err != nil {
			log.Fatalf("Error creating audio output file for video %s: %v\n", videoPath, err)
			continue
		}
		defer audioOutputFile.Close()

		videoOutputFile, err := os.Create(videoOutputFileName)
		if err != nil {
			log.Fatalf("Error creating video output file for video %s: %v\n", videoPath, err)
			continue
		}
		defer videoOutputFile.Close()
		fmt.Println("Output files created for video:", videoPath)

		fmt.Println("Chunking video sequentially...")
		// Pass the absolute videoPath to chunkVideo
		chunks, err := chunkVideo(videoPath, chunkDuration, videoIndex+1, baseName)
		if err != nil {
			log.Printf("Error chunking video %s: %v\n", videoPath, err)
			continue
		}
		fmt.Println("Video chunking complete.")

		fmt.Println("Processing video chunks in parallel...")
		// No more slices needed here

		for _, chunkData := range chunks {
			processChunk(chunkData, client, model, ctx, errorChannel, whisperCLIPath, whisperModelPath, whisperThreads, whisperLanguage, audioOutputFile, videoOutputFile)
		}

		fmt.Println("All video chunks processed. Sending combined prompt to LLM...")

		// Read the *entire* content of the audio and video files.
		audioContent, err := os.ReadFile(audioOutputFileName)
		if err != nil {
			log.Printf("Error reading audio output file: %v", err)
			continue // Crucial: Continue to the next video if reading fails
		}
		videoContent, err := os.ReadFile(videoOutputFileName)
		if err != nil {
			log.Printf("Error reading video output file: %v", err)
			continue
		}
		combinedAudioTranscript := string(audioContent) // Convert to string
		combinedVideoTranscript := string(videoContent)

		var promptTemplate string
		if inputFromUser != "" {
			promptTemplate = `Context from user about this video: %s
	Here is a raw transcription of a video. Your task is to refine it into a well-structured, human-like summary with explanations while keeping all the original details. Analyze the lecture provided in the audio transcription and video text. Identify the main topic, key arguments, supporting evidence, and any examples used, highlighting the connections between different ideas. Use information from both the audio transcription and video text to create a comprehensive explanation, also use timestamp to help us correlate with the audio transcript:

    --- RAW TRANSCRIPTION of Audio ---
    %s

    --- RAW TRANSCRIPTION of Video Text ---
    %s

    Please rewrite it clearly with explanations where needed, ensuring it's easy to read and understand.`
		} else {
			promptTemplate = `Here is a raw transcription of a video. Your task is to refine it into a well-structured, human-like summary with explanations while keeping all the original details. Analyze the lecture provided in the audio transcription and video text. Identify the main topic, key arguments, supporting evidence, and any examples used, highlighting the connections between different ideas. Use information from both the audio transcription and video text to create a comprehensive explanation, also use timestamp to help us correlate with the audio transcript:

    --- RAW TRANSCRIPTION of Audio ---
    %s

    --- RAW TRANSCRIPTION of Video Text ---
    %s

    Please rewrite it clearly with explanations where needed, ensuring it's easy to read and understand.`
		}

		combinedPromptText := fmt.Sprintf(promptTemplate, inputFromUser, combinedAudioTranscript, combinedVideoTranscript)

		combinedPrompt := []genai.Part{
			genai.Text(combinedPromptText),
		}

		sentLlmPrompt(model, combinedPrompt, ctx, outputFile, videoIndex+1) // Now passing the file
		fmt.Printf("\n--- FINISHED PROCESSING VIDEO %d: %s ---\n", videoIndex+1, videoPath)
		fmt.Fprintf(outputFile, "\n--- VIDEO %d PROCESSING COMPLETE ---\n\n", videoIndex+1)
		fmt.Fprintf(audioOutputFile, "\n--- VIDEO %d PROCESSING COMPLETE ---\n\n", videoIndex+1)
		fmt.Fprintf(videoOutputFile, "\n--- VIDEO %d PROCESSING COMPLETE ---\n\n", videoIndex+1)
	}
	close(errorChannel) // Close *after* the loop, *before* reading
	for err := range errorChannel {
		log.Println("Error from goroutine:", err)
	}

	fmt.Println("\nAll videos processing complete.")
	fmt.Println("Exiting.")
	return nil

}

func IsUrl(str string) string {
	u, err := url.Parse(str)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return "url"
	}
	return "path"
}

// main function

func main() {
	// Use all available CPUs

	if len(os.Args) != 9 {
		fmt.Println("Usage: program <llm_model> <api_key> <chunk_duration_seconds> <whisper_cli_path> <whisper_model_path> <whisper_threads> <whisper_language> <video_path_or_folder_or_youtube_url>")
		os.Exit(1)
	}
	llm := os.Args[1]
	apiKey := os.Args[2]
	chunkDuration, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid chunk duration: %v\n", err)
	}
	whisperCLIPath := os.Args[4]
	whisperModelPath := os.Args[5]
	whisperThreads, err := strconv.Atoi(os.Args[6])
	if err != nil {
		log.Fatalf("Invalid whisper threads: %v\n", err)
	}
	whisperLanguage := os.Args[7]
	inputPath := os.Args[8]

	if IsUrl(inputPath) == "url" {
		// Determine absolute destination directory
		currentDir, err := os.Executable()
		if err != nil {
			log.Fatalf("Error getting current directory: %v\n", err)
		}
		// Use getDestinationDir which now ensures absolute path and creates the dir
		destinationDir := getDestinationDir(filepath.Join(currentDir, "Videos"))
		// No need to MkdirAll here, getDestinationDir/YoutubeDownloader handles it

		log.Printf("Attempting download from URL: %s to Directory: %s\n", inputPath, destinationDir)
		// YoutubeDownloader now returns the guaranteed absolute path
		absPath, err := YoutubeDownloader(inputPath, destinationDir)
		if err != nil {
			log.Fatalf("Error downloading YouTube video: %v\n", err)
		}

		log.Printf("Download complete. Video saved at absolute path: %s\n", absPath)

		// Verify file exists at the absolute path returned
		if _, err := os.Stat(absPath); err != nil {
			// Use %v for the error to see more detail (e.g., os.IsNotExist)
			log.Fatalf("Downloaded video file not found or inaccessible at: %s. Error: %v\n", absPath, err)
		}

		log.Printf("File verified. Proceeding to process video: %s\n", absPath)

		// Pass the verified absolute path to VideoSummary
		err = VideoSummary(llm, apiKey, chunkDuration, whisperCLIPath, whisperModelPath, whisperThreads, whisperLanguage, absPath, "")
		if err != nil {
			log.Fatalf("Error in VideoSummary: %v\n", err)
		}

	} else if IsUrl(inputPath) == "path" {
		// For direct file paths, ensure we have absolute path
		absPath, err := filepath.Abs(inputPath)
		if err != nil {
			log.Fatalf("Error getting absolute path for input '%s': %v\n", inputPath, err)
		}

		// Verify the input file path
		if _, err := os.Stat(absPath); err != nil {
			log.Fatalf("Input video file not found or inaccessible at: %s. Error: %v\n", absPath, err)
		}

		fmt.Printf("Processing local video file: %s\n", absPath)
		err = VideoSummary(llm, apiKey, chunkDuration, whisperCLIPath, whisperModelPath, whisperThreads, whisperLanguage, absPath, "")
		if err != nil {
			log.Fatalf("Error in VideoSummary: %v\n", err)
		}
	}
}

// IsVideoFile function
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExtensions := []string{".mp4", ".mov", ".avi", ".wmv", ".mkv", ".flv", ".webm", ".mpeg", ".mpg"}
	for _, vext := range videoExtensions {
		if ext == vext {
			return true
		}
	}
	return false
}
