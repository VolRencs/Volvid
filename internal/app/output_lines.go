package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	commandLineBufferSize = 128 << 10
	commandLineMaxBytes   = 16 << 20
)

var ErrCommandLineTooLong = errors.New("command output line exceeds safe limit")

func readCommandLines(r io.Reader, handle func([]byte) error) error {
	if r == nil {
		return nil
	}

	reader := bufio.NewReaderSize(r, commandLineBufferSize)
	var line bytes.Buffer

	for {
		chunk, isPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line.Len() == 0 {
					return nil
				}
				return handle(trimCommandLine(line.Bytes()))
			}
			return err
		}

		if line.Len()+len(chunk) > commandLineMaxBytes {
			if drainErr := discardRemainingLine(reader, isPrefix); drainErr != nil {
				return errors.Join(fmt.Errorf("%w (%d bytes)", ErrCommandLineTooLong, commandLineMaxBytes), drainErr)
			}
			return fmt.Errorf("%w (%d bytes)", ErrCommandLineTooLong, commandLineMaxBytes)
		}

		if _, writeErr := line.Write(chunk); writeErr != nil {
			return writeErr
		}
		if isPrefix {
			continue
		}

		if err := handle(trimCommandLine(line.Bytes())); err != nil {
			return err
		}
		line.Reset()
	}
}

func discardRemainingLine(reader *bufio.Reader, isPrefix bool) error {
	for isPrefix {
		_, nextPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		isPrefix = nextPrefix
	}
	return nil
}

func trimCommandLine(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	return bytes.TrimSpace(line)
}
