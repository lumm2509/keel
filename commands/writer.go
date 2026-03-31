package commands

import (
	"io"
	"os"

	"github.com/fatih/color"
)

// NewColoredErrWriter returns an io.Writer that prints to stderr in red.
// It is used to colorize Cobra's error output.
func NewColoredErrWriter() io.Writer {
	return &coloredWriter{
		w: os.Stderr,
		c: color.New(color.FgRed),
	}
}

type coloredWriter struct {
	w io.Writer
	c *color.Color
}

func (cw *coloredWriter) Write(p []byte) (int, error) {
	cw.c.SetWriter(cw.w)
	defer cw.c.UnsetWriter(cw.w)
	return cw.c.Print(string(p))
}
