package app

import (
    "fmt"
    "gophertube/internal/services"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/urfave/cli/v3"
)

// ANSI colors and bar constants are defined in ui.go

// sanitizeFilename converts a video title into a filesystem-safe filename.
func sanitizeFilename(s string) string {
    s = strings.ReplaceAll(s, " ", "_")
    allowed := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
    return strings.Map(func(r rune) rune {
        if strings.ContainsRune(allowed, r) {
            return r
        }
        return '_'
    }, s)
}

// qualityToFormat maps a human-readable quality to yt-dlp/mpv format selectors.
func qualityToFormat(q string) string {
    switch q {
    case "1080p":
        return "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
    case "720p":
        return "bestvideo[height<=720]+bestaudio/best[height<=720]"
    case "480p":
        return "bestvideo[height<=480]+bestaudio/best[height<=480]"
    case "360p":
        return "bestvideo[height<=360]+bestaudio/best[height<=360]"
    case "Audio":
        return "bestaudio"
    default:
        return "best"
    }
}

// buildDownloadsPreview returns the fzf preview command for the downloads list.
func buildDownloadsPreview(downloadsPath string) string {
    const tpl = `sh -c 'file="$1"; base="%s/${file%%.*}"; thumb="$base.jpg"; w=$((FZF_PREVIEW_COLUMNS * 9 / 10)); h=$((FZF_PREVIEW_LINES * 3 / 5)); if [ -f "$thumb" ]; then chafa --size=${w}x${h} "$thumb" 2>/dev/null; else echo "No image preview available"; fi; echo; printf "\033[1;36m%s\033[0m\n" "$file"' sh {}`
    return fmt.Sprintf(tpl, downloadsPath)
}

// MediaPlayer represents available media players
type MediaPlayer struct {
    Name string
    Path string
}

// checkAvailablePlayer checks for MPV.
func checkAvailablePlayer() *MediaPlayer {
    // Prefer MPV for better performance and terminal integration
    if path, err := exec.LookPath("mpv"); err == nil {
        return &MediaPlayer{
            Name: "mpv",
            Path: path,
        }
    }
    return nil
}

// playWithPlayer plays media using the detected player.
func playWithPlayer(player *MediaPlayer, url string, isAudioOnly bool) error {
    var args []string

    if isAudioOnly {
        args = []string{"--no-video"}
    }

    args = append(args, url)

    cmd := exec.Command(player.Path, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}

func gophertubeYouTubeMode(cmd *cli.Command) {
    query, esc := readQuery()
    if esc || query == "" {
        fmt.Print("\033[2J\033[H")
        return
    }
    for {
        // Spinner/progress state
        progressCurrent := 0
        progressTotal := 1
        progressDone := make(chan struct{})

        // Start spinner goroutine
        go func() {
            for {
                select {
                case <-progressDone:
                    return
                default:
                    printProgressBar(progressCurrent, progressTotal)
                    time.Sleep(100 * time.Millisecond)
                }
            }
        }()

        videos, err := services.SearchYouTube(query, cmd.Int(FlagSearchLimit), func(current, total int) {
            progressCurrent = current
            progressTotal = total
        })

        close(progressDone)
        fmt.Print("\033[2K\r\n") // Clear progress bar/spinner line
        fmt.Println()
        fmt.Println()

        if err != nil || len(videos) == 0 {
            fmt.Println("    "+colorRed+"No results found."+colorReset)
            fmt.Println()
            fmt.Println("    "+colorWhite+"Press any key to search again..."+colorReset)
            os.Stdin.Read(make([]byte, 1))
            return
        }

        fmt.Printf("    %sFound %d results!%s\n", colorGreen, len(videos), colorReset)
        printSearchStats(videos)
        printSearchTips()
        // Reduced delay for faster response
        time.Sleep(200 * time.Millisecond)

        for {
            selected := runFzf(videos, cmd.Int(FlagSearchLimit), query)
            if selected == -2 {
                // User pressed escape, go back to new search
                return
            }
            if selected < 0 || selected >= len(videos) {
                continue // Stay in the same list
            }

            // Show Watch/Download/Audio menu
            menu := []string{"Watch", "Download", "Listen"}
            action := exec.Command("fzf", "--prompt=Action: ")
            action.Stdin = strings.NewReader(strings.Join(menu, "\n"))
            out, _ := action.Output()
            choice := strings.TrimSpace(string(out))

            if choice == "Download" {
                qualities := []string{"1080p", "720p", "480p", "360p", "Audio"}
                actionQ := exec.Command("fzf", "--prompt=Quality: ")
                actionQ.Stdin = strings.NewReader(strings.Join(qualities, "\n"))
                outQ, _ := actionQ.Output()
                selectedQ := strings.TrimSpace(string(outQ))
                if selectedQ != "" {
                    // Map quality to yt-dlp format
                    format := qualityToFormat(selectedQ)
                    os.MkdirAll(cmd.String(FlagDownloadsPath), 0755)
                    // Sanitize filename
                    filename := sanitizeFilename(videos[selected].Title)
                    outputPath := fmt.Sprintf("%s/%s.%%(ext)s", cmd.String(FlagDownloadsPath), filename)
                    fmt.Printf("    %sDownloading '%s' as %s...%s\n", colorGreen, videos[selected].Title, selectedQ, colorReset)

                    ytDlpArgs := []string{"-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}

                    //override the default args with an audio only version.
                    // Note: this downlads it as a .webm, then converts it to a .opus file.
                    if format == "bestaudio" {
                        ytDlpArgs = []string{"-x", "-f", format, "-o", outputPath, "--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg", videos[selected].URL}
                    }
                    actionDl := exec.Command("yt-dlp", ytDlpArgs...)
                    actionDl.Stdout = os.Stdout
                    actionDl.Stderr = os.Stderr
                    err := actionDl.Run()
                    if err == nil {
                        fmt.Printf("    %sDownload complete!%s\n", colorGreen, colorReset)
                        fmt.Printf("    %sSaved to: %s%s\n", colorWhite, cmd.String(FlagDownloadsPath), colorReset)
                    } else {
                        fmt.Printf("    %sDownload failed!%s\n", colorRed, colorReset)
                    }
                    fmt.Println("    "+colorWhite+"Press any key to return..."+colorReset)
                    os.Stdin.Read(make([]byte, 1))
                }
                gophertubeYouTubeMode(cmd)
                return
            }

            // New Audio playback logic
            if choice == "Listen" {
                player := checkAvailablePlayer()
                if player == nil {
                    fmt.Println("    "+colorRed+"No media player found!"+colorReset)
                    fmt.Println("    "+colorWhite+"Please install MPV to play audio."+colorReset)
                    fmt.Println("    "+colorYellow+"Install MPV: sudo apt install mpv (Ubuntu) | brew install mpv (macOS)"+colorReset)
                    fmt.Println("    "+colorWhite+"Press any key to return..."+colorReset)
                    os.Stdin.Read(make([]byte, 1))
                    continue // Go back to the search results
                }

                fmt.Printf("    %sPlaying Audio with %s: %s%s\n", colorYellow, strings.ToUpper(player.Name), videos[selected].Title, colorReset)
                fmt.Printf("    %sChannel: %s%s\n", colorWhite, videos[selected].Author, colorReset)
                fmt.Printf("    %sDuration: %s%s\n", colorWhite, videos[selected].Duration, colorReset)
                fmt.Printf("    %sPublished: %s%s\n", colorCyan, videos[selected].Published, colorReset)
                fmt.Println("    "+barMagenta)
                fmt.Println("    "+colorYellow+"Controls: 'q' to quit, SPACE to pause/resume, ←→ to seek"+colorReset)
                fmt.Println("    "+barMagenta)
                fmt.Println()

                // Extract direct audio stream URL
                audioCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
                streamURLBytes, err := audioCmd.Output()
                if err != nil {
                    fmt.Println("    "+colorRed+"Failed to get direct audio URL."+colorReset)
                    fmt.Println("    "+colorWhite+"Make sure yt-dlp is installed."+colorReset)
                    fmt.Println("    "+colorWhite+"Press any key to return..."+colorReset)
                    os.Stdin.Read(make([]byte, 1))
                    continue // Go back to the search results
                }
                streamURL := strings.TrimSpace(string(streamURLBytes))

                if err := playWithPlayer(player, streamURL, true); err != nil {
                    fmt.Printf("    \033[1;31mFailed to play audio with %s.\033[0m\n", player.Name)
                }

                fmt.Println("    "+colorWhite+"Press Enter to return."+colorReset)
                os.Stdin.Read(make([]byte, 1))
                continue // Return to the search results
            }

            // Watch as before
            fmt.Printf("    %sPlaying: %s%s\n", colorYellow, videos[selected].Title, colorReset)
            fmt.Printf("    %sChannel: %s%s\n", colorWhite, videos[selected].Author, colorReset)
            fmt.Printf("    %sDuration: %s%s\n", colorWhite, videos[selected].Duration, colorReset)
            fmt.Printf("    %sPublished: %s%s\n", colorCyan, videos[selected].Published, colorReset)
            fmt.Println()
            fmt.Println("    "+barMagenta)
            fmt.Println()
            mpvPath := "mpv"
            quality := cmd.String(FlagQuality)
            var mpvArgs []string

            // Add the fullscreen flag for video playback
            mpvArgs = append(mpvArgs, "--fs")

            if quality != "" {
                f := qualityToFormat(quality)
                if f == "bestaudio" {
                    mpvArgs = append(mpvArgs, "--no-video")
                }
                mpvArgs = append(mpvArgs, "--ytdl-format="+f)
            }

            mpvArgs = append(mpvArgs, videos[selected].URL)
            exec.Command(mpvPath, mpvArgs...).Run()
            continue
        }
    }
}

func gophertubeDownloadsMode(cmd *cli.Command) {
    files, err := os.ReadDir(cmd.String(FlagDownloadsPath))
    if err != nil || len(files) == 0 {
        fmt.Println("    "+colorRed+"No downloaded videos found."+colorReset)
        fmt.Println("    "+colorWhite+"Press any key to return to main menu..."+colorReset)
        os.Stdin.Read(make([]byte, 1))
        return
    }
    var videoFiles []string
    for _, f := range files {
        if !f.IsDir() && (strings.HasSuffix(f.Name(), ".mp4") || strings.HasSuffix(f.Name(), ".mkv") || strings.HasSuffix(f.Name(), ".webm") || strings.HasSuffix(f.Name(), ".avi")) {
            videoFiles = append(videoFiles, f.Name())
        }
    }
    if len(videoFiles) == 0 {
        fmt.Println("    "+colorRed+"No downloaded videos found."+colorReset)
        fmt.Println("    "+colorWhite+"Press any key to return to main menu..."+colorReset)
        os.Stdin.Read(make([]byte, 1))
        return
    }
    fzfPreview := buildDownloadsPreview(cmd.String(FlagDownloadsPath))
    action := exec.Command("fzf", "--ansi", "--preview-window=wrap", "--prompt=Downloads: ", "--preview", fzfPreview)
    action.Stdin = strings.NewReader(strings.Join(videoFiles, "\n"))
    out, _ := action.Output()
    selected := strings.TrimSpace(string(out))
    if selected == "" {
        return
    }
    filePath := cmd.String(FlagDownloadsPath) + "/" + selected
    fmt.Printf("    %sPlaying: %s%s\n", colorYellow, selected, colorReset)
    fmt.Println()
    fmt.Println("    "+barMagenta)
    fmt.Println()
    mpvPath := "mpv"
    exec.Command(mpvPath, filePath).Run()
}