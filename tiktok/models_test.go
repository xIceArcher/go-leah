package tiktok

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortByQuality(t *testing.T) {
	formats := RawFormats{
		{VideoCodec: "h264", Width: 40, Height: 30},
		{VideoCodec: "h265", Width: 20, Height: 20},
		{VideoCodec: "h264", Width: 50, Height: 40, FormatNote: "watermarked"},
		{VideoCodec: "h264", Width: 30, Height: 30, FormatNote: "watermarked"},
		{VideoCodec: "h265", Width: 50, Height: 40, FormatNote: "watermarked"},
		{VideoCodec: "h264", Width: 400, Height: 300},
	}

	expected := RawFormats{
		{VideoCodec: "h264", Width: 400, Height: 300},
		{VideoCodec: "h264", Width: 40, Height: 30},
		{VideoCodec: "h264", Width: 50, Height: 40, FormatNote: "watermarked"},
		{VideoCodec: "h264", Width: 30, Height: 30, FormatNote: "watermarked"},
		{VideoCodec: "h265", Width: 20, Height: 20},
		{VideoCodec: "h265", Width: 50, Height: 40, FormatNote: "watermarked"},
	}

	formats.SortByQuality()
	assert.EqualValues(t, expected, formats)
}
