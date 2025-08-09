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
			fmt.Println("    \033[1;31mNo results found.\033[0m")
			fmt.Println()
			fmt.Println("    \033[0;37mPress any key to search again...\033[0m")
			os.Stdin.Read(make([]byte, 1))
			return
		}

		fmt.Printf("    \033[1;32mFound %d results!\033[0m\n", len(videos))
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
			menu := []string{"Watch", "Download", "Audio"}
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
					// Real download logic
					format := "best"
					switch selectedQ {
					case "1080p":
						format = "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
					case "720p":
						format = "bestvideo[height<=720]+bestaudio/best[height<=720]"
					case "480p":
						format = "bestvideo[height<=480]+bestaudio/best[height<=480]"
					case "360p":
						format = "bestvideo[height<=360]+bestaudio/best[height<=360]"
					case "Audio":
						format = "bestaudio"
					}
					os.MkdirAll(cmd.String(FlagDownloadsPath), 0755)
					// Sanitize filename
					filename := strings.ReplaceAll(videos[selected].Title, " ", "_")
					filename = strings.Map(func(r rune) rune {
						if strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
							return r
						}
						return '_'
					}, filename)
					outputPath := fmt.Sprintf("%s/%s.%%(ext)s", cmd.String(FlagDownloadsPath), filename)
					fmt.Printf("    \033[1;32mDownloading '%s' as %s...\033[0m\n", videos[selected].Title, selectedQ)

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
						fmt.Printf("    \033[1;32mDownload complete!\033[0m\n")
						fmt.Printf("    \033[0;37mSaved to: %s\033[0m\n", cmd.String(FlagDownloadsPath))
					} else {
						fmt.Printf("    \033[1;31mDownload failed!\033[0m\n")
					}
					fmt.Println("    \033[0;37mPress any key to return...\033[0m")
					os.Stdin.Read(make([]byte, 1))
				}
				gophertubeYouTubeMode(cmd)
				return
			}

			// New Audio playback logic
			if choice == "Audio" {
				player := checkAvailablePlayer()
				if player == nil {
					fmt.Println("    \033[1;31mNo media player found!\033[0m")
					fmt.Println("    \033[0;37mPlease install MPV to play audio.\033[0m")
					fmt.Println("    \033[0;33mInstall MPV: sudo apt install mpv (Ubuntu) | brew install mpv (macOS)\033[0m")
					fmt.Println("    \033[0;37mPress any key to return...\033[0m")
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}

				fmt.Printf("    \033[1;33mPlaying Audio with %s: %s\033[0m\n", strings.ToUpper(player.Name), videos[selected].Title)
				fmt.Printf("    \033[0;37mChannel: %s\033[0m\n", videos[selected].Author)
				fmt.Printf("    \033[0;37mDuration: %s\033[0m\n", videos[selected].Duration)
				fmt.Printf("    \033[0;36mPublished: %s\033[0m\n", videos[selected].Published)
				fmt.Println("    \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
				fmt.Println("    \033[0;33mControls: 'q' to quit, SPACE to pause/resume, ←→ to seek\033[0m")
				fmt.Println("    \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n")

				// Extract direct audio stream URL
				audioCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio", "-g", videos[selected].URL)
				streamURLBytes, err := audioCmd.Output()
				if err != nil {
					fmt.Println("    \033[1;31mFailed to get direct audio URL.\033[0m")
					fmt.Println("    \033[0;37mMake sure yt-dlp is installed.\033[0m")
					fmt.Println("    \033[0;37mPress any key to return...\033[0m")
					os.Stdin.Read(make([]byte, 1))
					continue // Go back to the search results
				}
				streamURL := strings.TrimSpace(string(streamURLBytes))

				if err := playWithPlayer(player, streamURL, true); err != nil {
					fmt.Printf("    \033[1;31mFailed to play audio with %s.\033[0m\n", player.Name)
				}

				fmt.Println("    \033[0;37mPress Enter to return.\033[0m")
				os.Stdin.Read(make([]byte, 1))
				continue // Return to the search results
			}

			// Watch as before
			fmt.Printf("    \033[1;33mPlaying: %s\033[0m\n", videos[selected].Title)
			fmt.Printf("    \033[0;37mChannel: %s\033[0m\n", videos[selected].Author)
			fmt.Printf("    \033[0;37mDuration: %s\033[0m\n", videos[selected].Duration)
			fmt.Printf("    \033[0;36mPublished: %s\033[0m\n", videos[selected].Published)
			fmt.Println()
			fmt.Println("    \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
			fmt.Println()
			mpvPath := "mpv"
			quality := cmd.String(FlagQuality)
			var mpvArgs []string

			// Add the fullscreen flag for video playback
			mpvArgs = append(mpvArgs, "--fs")

			if quality != "" {
				// Map quality to ytdl-format string
				switch quality {
				case "1080p":
					mpvArgs = append(mpvArgs, "--ytdl-format="+"bestvideo[height<=1080]+bestaudio/best[height<=1080]")
				case "720p":
					mpvArgs = append(mpvArgs, "--ytdl-format="+"bestvideo[height<=720]+bestaudio/best[height<=720]")
				case "480p":
					mpvArgs = append(mpvArgs, "--ytdl-format="+"bestvideo[height<=480]+bestaudio/best[height<=480]")
				case "360p":
					mpvArgs = append(mpvArgs, "--ytdl-format="+"bestvideo[height<=360]+bestaudio/best[height<=360]")
				case "Audio":
					mpvArgs = append(mpvArgs, "--no-video")
					mpvArgs = append(mpvArgs, "--ytdl-format="+"bestaudio")
				default:
					mpvArgs = append(mpvArgs, "--ytdl-format="+"best")
				}
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
		fmt.Println("    \033[1;31mNo downloaded videos found.\033[0m")
		fmt.Println("    \033[0;37mPress any key to return to main menu...\033[0m")
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
		fmt.Println("    \033[1;31mNo downloaded videos found.\033[0m")
		fmt.Println("    \033[0;37mPress any key to return to main menu...\033[0m")
		os.Stdin.Read(make([]byte, 1))
		return
	}
	fzfPreview := fmt.Sprintf(`env file={} base="%s/${file%%.*}" thumb="$base.jpg" w=$((FZF_PREVIEW_COLUMNS * 9 / 10)) h=$((FZF_PREVIEW_LINES * 3 / 5)) sh -c '[ -f "$thumb" ] && chafa --size=${w}x${h} "$thumb" 2>/dev/null || echo "No image preview available" || echo "No thumbnail available"'`, cmd.String(FlagDownloadsPath))
	action := exec.Command("fzf", "--prompt=Downloads: ", "--preview", fzfPreview)
	action.Stdin = strings.NewReader(strings.Join(videoFiles, "\n"))
	out, _ := action.Output()
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return
	}
	filePath := cmd.String(FlagDownloadsPath) + "/" + selected
	fmt.Printf("    \033[1;33mPlaying: %s\033[0m\n", selected)
	fmt.Println()
	fmt.Println("    \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println()
	mpvPath := "mpv"
	exec.Command(mpvPath, filePath).Run()
}