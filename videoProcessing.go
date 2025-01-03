package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	buf := bytes.Buffer{}
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	vidSpecs := struct {
		Streams []struct {
			Width  float64 `json:"width,omitempty"`
			Height float64 `json:"height,omitempty"`
		} `json:"streams"`
	}{}

	err = json.Unmarshal(buf.Bytes(), &vidSpecs)
	if err != nil {
		return "", err
	}

	width := vidSpecs.Streams[0].Width
	height := vidSpecs.Streams[0].Height

	if width > height {
		return "16:9", nil
	} else if width < height {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	// new filename for output
	outputFilePath := filePath + ".processing"

	// run ffmpeg to add faststart
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	buf := bytes.Buffer{}
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputFilePath, nil

}
