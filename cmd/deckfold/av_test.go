package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/media"
)

// avDeck writes an HTML deck with a Windows Media video and a WAV sound, made with the ffmpeg
// of the machine, and returns the HTML file. It skips the test without ffmpeg.
func avDeck(t *testing.T) string {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil || !media.New().FFmpeg() {
		t.Skip("ffmpeg and ffprobe are not installed, or " + media.NoFFmpegEnv + " hides them")
	}
	dir := t.TempDir()
	lavfi := []string{"-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10:duration=1", "-f", "lavfi", "-i", "sine=frequency=440:duration=1"}
	for name, args := range map[string][]string{
		"clip.wmv": {"-c:v", "wmv2", "-c:a", "wmav2"},
		"tone.wav": {"-map", "1:a", "-c:a", "pcm_s16le"},
	} {
		cmd := exec.Command(ff, append(append(append([]string{}, lavfi...), args...), filepath.Join(dir, name))...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("this ffmpeg cannot make %s: %v %s", name, err, b)
		}
	}
	html := filepath.Join(dir, "d.html")
	put(t, html, `<!doctype html><html><head><meta charset="utf-8"><title>AV</title></head><body>
<video src="clip.wmv"></video><audio><source type="audio/wav" src="tone.wav"></audio></body></html>`)
	return html
}

// TestVideoAndSoundInHTML: with ffmpeg installed, a video and a sound that an HTML export plays
// are converted, the type of the <source> follows, and the report names them (decision E18).
func TestVideoAndSoundInHTML(t *testing.T) {
	html := avDeck(t)
	c, o, e := runCLI(t, html, "--generic")
	if c != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	if !strings.Contains(o, "Converted: 1 video and 1 sound, ") {
		t.Errorf("the report does not name the converted video and sound:\n%s", o)
	}
	mimes, sourceType, _, _ := mediaOutput(t, html)
	if mimes["video/mp4"] != 1 || mimes["audio/flac"] != 1 || len(mimes) != 2 {
		t.Errorf("asset blocks by type: %v", mimes)
	}
	if sourceType != "audio/flac" {
		t.Errorf(`<source type=%q>, want "audio/flac"`, sourceType)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(html), cacheFolder)); err == nil {
		t.Error("a cache folder was made, but video and sound are not kept")
	}

	// the switch leaves both as they are, and so does a machine without ffmpeg
	for how, set := range map[string]func() []string{
		"--keep-assets":  func() []string { return []string{html, "--generic", "--force", "--keep-assets"} },
		"without ffmpeg": func() []string { t.Setenv(media.NoFFmpegEnv, "1"); return []string{html, "--generic", "--force"} },
	} {
		c, o, e := runCLI(t, set()...)
		if c != 0 {
			t.Fatalf("%s: exit %d %s", how, c, e)
		}
		if strings.Contains(o, "Converted:") {
			t.Errorf("%s: the report names converted assets:\n%s", how, o)
		}
		if how == "without ffmpeg" && !strings.Contains(o+e, "ffmpeg is not installed") {
			t.Errorf("%s: the report does not say that ffmpeg would convert the video:\n%s%s", how, o, e)
		}
		mimes, sourceType, _, _ := mediaOutput(t, html)
		if mimes["video/mp4"] != 0 || mimes["audio/flac"] != 0 || len(mimes) != 2 || sourceType != "audio/wav" {
			t.Errorf("%s: asset blocks by type: %v, <source type=%q>", how, mimes, sourceType)
		}
	}
}
