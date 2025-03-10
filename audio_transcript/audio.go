package audio_transcript

import (
	"fmt"
	"github.com/ggerganov/whisper.cpp/bindings/go"
	"github.com/go-audio/wav"
	"os"
	"time"
)

func TranscribeAudio(audioFilePath string, ModelPath string) {
	// Open samples
	fh, _ := os.Open(audioFilePath)
	defer func(fh *os.File) {
		err := fh.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(fh)

	// Read samples
	d := wav.NewDecoder(fh)
	buf, _ := d.FullPCMBuffer()

	// Run whisper
	ctx := whisper.Whisper_init(ModelPath)

	defer ctx.Whisper_free()
	params := ctx.Whisper_full_default_params(whisper.SAMPLING_GREEDY)
	data := buf.AsFloat32Buffer().Data
	_ = ctx.Whisper_full(params, data, nil, nil, nil)

	// Print out tokens
	numSegments := ctx.Whisper_full_n_segments()

	for i := 0; i < numSegments; i++ {
		str := ctx.Whisper_full_get_segment_text(i)

		t0 := time.Duration(ctx.Whisper_full_get_segment_t0(i)) * time.Millisecond
		t1 := time.Duration(ctx.Whisper_full_get_segment_t1(i)) * time.Millisecond
		fmt.Printf("[%6s->%-6s] %q", t0, t1, str)
	}
}
