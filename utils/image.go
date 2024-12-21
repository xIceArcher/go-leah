package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func ConvertMP4ToGIF(video []byte) ([]byte, error) {
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

	ffprobeRawOutput, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", tmpVideoIn.Name()).Output()
	if err != nil {
		return nil, err
	}

	type FFProbeOutput struct {
		Streams []*struct {
			Width int `json:"width"`
		} `json:"streams"`
	}

	var ffprobeOutput *FFProbeOutput
	if err := json.Unmarshal(ffprobeRawOutput, &ffprobeOutput); err != nil {
		return nil, err
	}
	if len(ffprobeOutput.Streams) == 0 {
		return nil, fmt.Errorf("no streams detected")
	}

	videoWidth := ffprobeOutput.Streams[0].Width

	if err := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-vf", fmt.Sprintf("fps=15,scale=%v:-1:flags=lanczos,palettegen", videoWidth), "-y", tmpPalette.Name()).Run(); err != nil {
		return nil, err
	}

	if err := exec.Command("ffmpeg", "-i", tmpVideoIn.Name(), "-i", tmpPalette.Name(), "-lavfi", fmt.Sprintf("fps=15,scale=%v:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=sierra3", videoWidth), "-y", tmpGIFOut.Name()).Run(); err != nil {
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
