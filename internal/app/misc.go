package app

import (
	"bytes"
	"fmt"
	"gophertube/internal/services"
	"gophertube/internal/types"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

func printBanner() {
	fmt.Print("\033[2J\033[H")
	fmt.Println()
	fmt.Println("    \033[1;33mGopherTube\033[0m")
	fmt.Println("    \033[0;37mversion " + version + "\033[0m")
	fmt.Println()
	fmt.Println("    \033[1;36mFast Youtube Terminal UI\033[0m")
	fmt.Println("    \033[0;37mPress Ctrl+C or Esc to exit\033[0m")
	fmt.Println()
	fmt.Println("    \033[1;35m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println()
}

func printSearchPrompt(query string) {
	fmt.Print("\033[2K\r")
	fmt.Print("    \033[1;32m>\033[0m ")
	fmt.Print("\033[1;37m" + query + "\033[0m")
	fmt.Print("\033[1;30m█\033[0m")
}

func printProgressBar(current, total int) {
	width := 40
	filled := (current * width) / total
	percentage := (current * 100) / total

	// Create animated progress bar with original cyan color
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "\033[1;36m█" // Cyan for filled (original color)
		} else {
			bar += "\033[0;37m░"
		}
	}
	bar += "\033[0m"

	// Add spinning animation
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinners[time.Now().UnixNano()/100000000%int64(len(spinners))]

	// Format percentage with proper padding
	percentStr := fmt.Sprintf("%3d%%", percentage)

	fmt.Printf("\033[2K\r    %s %s %s", spinner, bar, percentStr)
}

func printSearchStats(videos []types.Video) {
	if len(videos) == 0 {
		return
	}

	channels := make(map[string]int)
	hasDuration := 0

	for _, v := range videos {
		channels[v.Author]++
		if v.Duration != "" {
			hasDuration++
		}
	}

	// Calculate average duration if available
	var avgDuration string
	if hasDuration > 0 {
		avgDuration = "~" + videos[0].Duration // Simple approximation
	}

	fmt.Println("    \033[1;36mSearch Statistics:\033[0m")
	fmt.Printf("    \033[0;37m• Total videos found: \033[1;32m%d\033[0m\n", len(videos))
	fmt.Printf("    \033[0;37m• Unique channels: \033[1;33m%d\033[0m\n", len(channels))

	if avgDuration != "" {
		fmt.Printf("    \033[0;37m• Average duration: \033[1;35m%s\033[0m\n", avgDuration)
	}

	// Show top channels if there are multiple
	if len(channels) > 1 && len(videos) > 3 {
		fmt.Printf("    \033[0;37m• Most active channel: \033[1;31m%s\033[0m\n", getTopChannel(channels))
	}

	fmt.Println()
}

func getTopChannel(channels map[string]int) string {
	var topChannel string
	maxCount := 0

	for channel, count := range channels {
		if count > maxCount {
			maxCount = count
			topChannel = channel
		}
	}

	if len(topChannel) > 30 {
		return topChannel[:27] + "..."
	}
	return topChannel
}

func printSearchTips() {
	tips := []string{
		"Tip: Press Tab to load more results, Esc to go back",
		"Tip: Use ↑/↓ to navigate, Enter to select, Ctrl+C to exit",
	}

	randomTip := tips[time.Now().Unix()%int64(len(tips))]
	fmt.Printf("    \033[1;33m%s\033[0m\n", randomTip)
	fmt.Println()
}

func readQuery() (string, bool) {
	printBanner()
	fmt.Print("    \033[1;32m>\033[0m ")

	// Use raw terminal mode for proper key detection
	oldState, err := readline.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", false
	}
	defer readline.Restore(int(os.Stdin.Fd()), oldState)

	var query []rune
	buf := make([]byte, 4)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		// Handle escape sequences
		if buf[0] == 27 {
			if n == 1 {
				// Just Escape key pressed
				return "", true
			}
			continue
		}

		// Handle special keys
		if buf[0] == 13 { // Enter
			fmt.Println()
			break
		}
		if buf[0] == 3 { // Ctrl+C
			return "", true
		}
		if buf[0] == 127 && len(query) > 0 { // Backspace
			query = query[:len(query)-1]
			fmt.Print("\033[D \033[D")
		} else if buf[0] >= 32 && buf[0] < 127 { // Printable characters
			query = append(query, rune(buf[0]))
			fmt.Printf("%c", buf[0])
		}
	}

	return string(query), false
}

func runFzf(videos []types.Video, searchLimit int, query string) int {
	limit := searchLimit
	filter := ""
	for {
		var input bytes.Buffer
		for i, v := range videos {
			thumbPath := v.ThumbnailPath
			thumbPath = strings.ReplaceAll(thumbPath, "'", "'\\''")
			fmt.Fprintf(&input, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", i, v.Title, thumbPath, v.Duration, v.Author, v.Views, v.Description, v.Published)
		}
		fzfArgs := []string{
			"--with-nth=2..2",
			"--delimiter=\t",
			fmt.Sprintf("--header=\033[1;36m↑/↓\033[0m to move • \033[1;33mtype\033[0m to search • \033[1;32mEnter\033[0m to select • \033[1;35mTab\033[0m to load more • \033[1;37m%d results • \033[1;35m%s\033[0m", len(videos), query),
			"--expect=tab",
			"--bind=esc:abort",
			"--border=rounded",
			"--margin=1,1",
			"--preview",
			`sh -c 'thumbfile="$1"; title="$2"; w=$((FZF_PREVIEW_COLUMNS * 9 / 10)); h=$((FZF_PREVIEW_LINES * 3 / 5)); if [ -s "$thumbfile" ] && [ -f "$thumbfile" ]; then chafa --size=${w}x${h} "$thumbfile" 2>/dev/null; else echo "No image preview available"; fi; echo; printf "\033[1;37m%s\033[0m\n" "$title"; printf "\033[33mDuration:\033[0m %s\n" "$3"; printf "\033[36mPublished:\033[0m %s\n" "$4"; printf "\033[32mAuthor:\033[0m %s\n" "$5"; printf "\033[35mViews:\033[0m %s\n" "$6"' sh {3} {2} {4} {8} {5} {6}`,
		}
		if filter != "" {
			fzfArgs = append(fzfArgs, "--query="+filter)
		}
		cmd := exec.Command("fzf", fzfArgs...)
		cmd.Stdin = &input
		pr, pw, _ := os.Pipe()
		cmd.Stdout = pw
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Println("\033[1;31mfzf error:\033[0m", err)
			return -1
		}
		pw.Close()
		selected, _ := io.ReadAll(pr)
		cmd.Wait()
		lines := strings.Split(strings.TrimSpace(string(selected)), "\n")
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			return -2 // user pressed escape in fzf
		}
		if lines[0] == "tab" {
			fmt.Printf("    \033[1;35mLoading more results...\033[0m\n")
			limit += searchLimit
			moreVideos, err := services.SearchYouTube(query, limit, func(current, total int) {
				// progressCurrent = current // This line was removed
				// progressTotal = total // This line was removed
			})
			if err != nil || len(moreVideos) == len(videos) {
				continue
			}
			videos = moreVideos
			fmt.Printf("    \033[1;32mLoaded %d total results!\033[0m\n", len(videos))
			printSearchStats(videos)
			continue
		}
		line := lines[0]
		if line == "" {
			return -1
		}
		idxStr := strings.SplitN(line, "\t", 2)[0]
		idx := 0
		fmt.Sscanf(idxStr, "%d", &idx)
		return idx
	}
}
