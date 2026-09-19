package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AvengeMedia/dankgo/lyrics"
)

func main() {
	artist, song, err := readArtistAndSong(os.Stdin)
	if err != nil {
		if len(os.Args) >= 3 {
			artist = strings.TrimSpace(os.Args[1])
			song = strings.TrimSpace(os.Args[2])
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  echo -e \"<artist>\\n<song>\" | go run ./cmd/lyrics-test")
			fmt.Fprintln(os.Stderr, "  echo \"<artist> - <song>\" | go run ./cmd/lyrics-test")
			fmt.Fprintln(os.Stderr, "  go run ./cmd/lyrics-test \"<artist>\" \"<song>\"")
			os.Exit(1)
		}
	}

	if artist == "" || song == "" {
		fmt.Fprintln(os.Stderr, "Error: both artist and song must be specified")
		os.Exit(1)
	}

	client := lyrics.New(lyrics.Options{
		UserAgent: "music-player-lyrics-test/1.0 (+https://github.com/KotonBads/mosaic)",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req := lyrics.Request{
		Artist: artist,
		Title:  song,
	}

	result, err := client.Lookup(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lookup error: %v\n", err)
		if result == nil {
			os.Exit(1)
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func readArtistAndSong(r io.Reader) (string, string, error) {
	if f, ok := r.(*os.File); ok && isTerminal(f) {
		scanner := bufio.NewScanner(r)
		fmt.Fprint(os.Stderr, "Enter artist: ")
		if !scanner.Scan() {
			return "", "", scanner.Err()
		}
		artist := strings.TrimSpace(scanner.Text())

		fmt.Fprint(os.Stderr, "Enter song: ")
		if !scanner.Scan() {
			return "", "", scanner.Err()
		}
		song := strings.TrimSpace(scanner.Text())

		if artist == "" || song == "" {
			return "", "", fmt.Errorf("artist or song cannot be empty")
		}
		return artist, song, nil
	}

	// Non-terminal or piped stdin:
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	if len(lines) == 0 {
		return "", "", fmt.Errorf("no input provided on stdin")
	}

	if len(lines) >= 2 {
		return lines[0], lines[1], nil
	}

	// Single line provided: attempt to parse delimiters
	line := lines[0]

	// 1. Check for standard separators: " - ", " – ", " — "
	for _, sep := range []string{" - ", " \u2013 ", " \u2014 ", "\t", " | "} {
		if strings.Contains(line, sep) {
			parts := strings.SplitN(line, sep, 2)
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
		}
	}

	// 2. Check for quoted words: e.g. "Daft Punk" "Get Lucky"
	quoteRe := regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
	matches := quoteRe.FindAllStringSubmatch(line, -1)
	if len(matches) == 2 {
		val1 := matches[0][1]
		if val1 == "" {
			val1 = matches[0][2]
		}
		val2 := matches[1][1]
		if val2 == "" {
			val2 = matches[1][2]
		}
		return strings.TrimSpace(val1), strings.TrimSpace(val2), nil
	}

	// 3. Comma separator: "Artist, Song"
	if strings.Contains(line, ",") {
		parts := strings.SplitN(line, ",", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}

	// 4. Two single words separated by space: "Adele Hello"
	parts := strings.Fields(line)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("could not determine artist and song from single line: %q", line)
}
