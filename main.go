package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// setLlmApi function
func setLlmApi(llm string, apiKey string) (*genai.Client, *genai.GenerativeModel, context.Context) {
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
	maxRetries = 3                // Maximum number of retry attempts for LLM calls
	retryDelay = 15 * time.Second // Delay between retry attempts
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

	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
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

// transcribeAudioWhisperCLI function
func transcribeAudioWhisperCLI(audioPath string, whisperCLIPath string, whisperModelPath string, videoIndex int, chunkNum int, threads int, language string) (string, error) {
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

// transcribeFramesTesseractCLI function
func transcribeFramesTesseractCLI(framePaths []string) (string, error) {
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
		transcript, err := transcribeFramesTesseractCLI(framePaths)
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
	time.Sleep(30 * time.Second) // Wait for file to be ready
	defer func() { client.DeleteFile(ctx, uploadedFile.Name) }()

	fmt.Printf("Chunk %d for video %d: Video chunk uploaded as: %s\n", chunkNum, videoIndex, uploadedFile.URI)

	promptList := []genai.Part{
		genai.Text("## Task Description\nAnalyze the video and provide a detailed raw transcription of text displayed in the video."),
		genai.FileData{URI: uploadedFile.URI},
	}
	videoTranscript := sentLlmPrompt(model, promptList, ctx, nil, videoIndex) // No file writing here

	if videoTranscript == "" {
		// If LLM transcription fails, fall back to Tesseract
		fmt.Printf("Chunk %d for video %d: LLM transcription failed, falling back to Tesseract...\n", chunkNum, videoIndex)
		framePaths, err := extractFrames(videoPath, videoIndex, chunkNum)
		if err != nil {
			return "", fmt.Errorf("error extracting frames for video %d chunk %d: %w", videoIndex, chunkNum, err)
		}
		transcript, err := transcribeFramesTesseractCLI(framePaths)
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
		audioTranscript, audioErr = transcribeAudioWhisperCLI(chunk.AudioPath, whisperCLIPath, whisperModelPath, chunk.VideoIndex, chunk.ChunkNum, whisperThreads, whisperLanguage)
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

// main function
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()) // Use all available CPUs

	if len(os.Args) != 9 {
		fmt.Println("Usage: program <llm_model> <api_key> <chunk_duration_seconds> <whisper_cli_path> <whisper_model_path> <whisper_threads> <whisper_language> <video_path_or_folder>")
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

	client, model, ctx := setLlmApi(llm, apiKey)
	defer client.Close()

	errorChannel := make(chan error, 10) // Buffered channel

	var videoPaths []string
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		log.Fatalf("Error accessing input path: %v\n", err)
	}

	if fileInfo.IsDir() {
		fmt.Println("Processing folder:", inputPath)
		err = filepath.WalkDir(inputPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error walking directory: %v\n", err)
		}
	} else {
		fmt.Println("Processing single file:", inputPath)
		if isVideoFile(inputPath) {
			videoPaths = append(videoPaths, inputPath)
		} else {
			log.Println("Warning: Input path is not a video file:", inputPath)
		}
	}

	if len(videoPaths) == 0 {
		fmt.Println("No video files found to process.")
		return
	}

	for videoIndex, videoPath := range videoPaths {
		baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
		outputFileName := baseName + "_output.txt"
		audioOutputFileName := baseName + "_audio_output.txt"
		videoOutputFileName := baseName + "_video_output.txt"

		fmt.Printf("\n--- START PROCESSING VIDEO %d: %s ---\n", videoIndex+1, videoPath)
		fmt.Println("Creating output files for video:", videoPath)

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

		combinedPromptText := fmt.Sprintf(`Here is a raw transcription of a video. Your task is to refine it into a well-structured, human-like summary with explanations while keeping all the original details. Analyze the lecture provided in the audio transcription and video text.  Identify the main topic, key arguments, supporting evidence, and any examples used.  Explain the lecture in a structured way, highlighting the connections between different ideas.  Use information from both the audio transcription and video text to create a comprehensive explanation, also use timestamp to help us correlate with the audio transcript:

    --- RAW TRANSCRIPTION of Audio ---
    %s

    --- RAW TRANSCRIPTION of Video Text ---
    %s

    Please rewrite it clearly with explanations where needed, ensuring it's easy to read and understand.`, combinedAudioTranscript, combinedVideoTranscript)

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
}

// isVideoFile function
func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExtensions := []string{".mp4", ".mov", ".avi", ".wmv", ".mkv", ".flv", ".webm", ".mpeg", ".mpg"}
	for _, vext := range videoExtensions {
		if ext == vext {
			return true
		}
	}
	return false
}
