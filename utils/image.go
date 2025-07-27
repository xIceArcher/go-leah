package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"go.uber.org/zap"
)

func ConvertMP4ToGIF(video []byte, maxBytesInt64 int64) ([]byte, error) {
	maxBytes := int(maxBytesInt64)

	if len(video) > maxBytes {
		return nil, fmt.Errorf("original video is too large to convert")
	}

	losslessGIF, err := ConvertMP4ToGIFLossless(video)
	if err == nil && len(losslessGIF) <= maxBytes {
		zap.S().Info("Sent compressed GIF")
		return losslessGIF, nil
	}

	// Either we failed to convert to a lossless GIF or the lossless GIF is too big
	// Try a compressed version
	compressedGIF, err := ConvertMP4ToGIFCompressed(video)
	if err != nil {
		return nil, err
	} else if len(compressedGIF) > maxBytes {
		return nil, fmt.Errorf("compressed gif is too large")
	}

	return compressedGIF, nil
}

func ConvertMP4ToGIFLossless(video []byte) ([]byte, error) {
	tmpVideoIn, removeFunc, err := createAndCloseTempFile("*.mp4")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	tmpPalette, removeFunc, err := createAndCloseTempFile("*.png")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	tmpGIFOut, removeFunc, err := createAndCloseTempFile("*.gif")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(tmpVideoIn.Name(), video, 0644); err != nil {
		return nil, err
	}

	ffprobeOutput, err := ffprobeFile(tmpVideoIn.Name())
	if err != nil {
		return nil, err
	}

	frameOutFormat := fmt.Sprintf("%s/%%04d.png", dir)

	cmd1 := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-vsync", "0", "-q:v", "1", frameOutFormat)
	if err := cmd1.Run(); err != nil {
		return nil, err
	}

	cmd2 := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-vf", "palettegen", "-y", tmpPalette.Name())
	if err := cmd2.Run(); err != nil {
		return nil, err
	}

	cmd3 := exec.Command("ffmpeg", "-framerate", fmt.Sprintf("%.02f", float64(ffprobeOutput.NumFrames)/ffprobeOutput.Duration.Seconds()), "-i", frameOutFormat, "-i", tmpPalette.Name(), "-lavfi", "paletteuse=dither=sierra3", "-y", tmpGIFOut.Name())
	if err := cmd3.Run(); err != nil {
		return nil, err
	}

	r, err := os.Open(tmpGIFOut.Name())
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}

func ConvertMP4ToGIFCompressed(video []byte) ([]byte, error) {
	tmpVideoIn, removeFunc, err := createAndCloseTempFile("*.mp4")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	tmpPalette, removeFunc, err := createAndCloseTempFile("*.png")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	tmpGIFOut, removeFunc, err := createAndCloseTempFile("*.gif")
	if err != nil {
		return nil, err
	}
	defer removeFunc()

	if err := os.WriteFile(tmpVideoIn.Name(), video, 0644); err != nil {
		return nil, err
	}

	ffprobeOutput, err := ffprobeFile(tmpVideoIn.Name())
	if err != nil {
		return nil, err
	}

	if err := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-vf", fmt.Sprintf("fps=15,scale=%v:-1:flags=lanczos,palettegen", ffprobeOutput.Width), "-y", tmpPalette.Name()).Run(); err != nil {
		return nil, err
	}

	if err := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-i", tmpPalette.Name(), "-lavfi", fmt.Sprintf("fps=15,scale=%v:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=sierra3", ffprobeOutput.Width), "-y", tmpGIFOut.Name()).Run(); err != nil {
		return nil, err
	}

	r, err := os.Open(tmpGIFOut.Name())
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}

func createAndCloseTempFile(pattern string) (f *os.File, remove func() error, err error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, nil, err
	}

	if err := file.Close(); err != nil {
		return nil, nil, err
	}

	return file, func() error { return os.Remove(file.Name()) }, nil
}

type FFProbeOutput struct {
	Width     int
	Duration  time.Duration
	NumFrames int
}

func ffprobeFile(fileName string) (*FFProbeOutput, error) {
	ffprobeRawOutputBytes, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", fileName).Output()
	if err != nil {
		return nil, err
	}

	type FFProbeRawOutput struct {
		Streams []*struct {
			Width    int    `json:"width"`
			NBFrames string `json:"nb_frames"`
			Duration string `json:"duration"`
		} `json:"streams"`
	}

	var ffprobeRawOutput *FFProbeRawOutput
	if err := json.Unmarshal(ffprobeRawOutputBytes, &ffprobeRawOutput); err != nil {
		return nil, err
	}

	if len(ffprobeRawOutput.Streams) == 0 {
		return nil, fmt.Errorf("no streams detected")
	}

	numFrames, err := strconv.Atoi(ffprobeRawOutput.Streams[0].NBFrames)
	if err != nil {
		return nil, err
	}

	durationSecs, err := strconv.ParseFloat(ffprobeRawOutput.Streams[0].Duration, 64)
	if err != nil {
		return nil, err
	}

	return &FFProbeOutput{
		Width:     ffprobeRawOutput.Streams[0].Width,
		Duration:  time.Duration(durationSecs*1000000000) * time.Nanosecond,
		NumFrames: numFrames,
	}, nil
}
