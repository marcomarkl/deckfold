package pptx

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"deckfold/internal/htmlscan"
	"deckfold/internal/media"
	"deckfold/internal/pptx/pptxgen"
)

// convertMedia converts a generated deck with the conversion of pictures and fonts switched on
// and returns the folder of the deck and its HTML.
func convertMedia(t *testing.T, d *pptxgen.Deck) (string, *htmlscan.Doc, *media.Optimizer) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "in.pptx")
	if err := d.Write(src); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deck")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	m := media.New()
	if _, err := Convert(src, out, Options{BundledFontsOnly: true, Media: m}); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return out, htmlscan.Scan(html), m
}

// asset reads the file an href or src of the deck points to.
func asset(t *testing.T, dir, ref string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestMediaConversion: with an optimizer, the pictures of a deck are WebP files and its fonts
// WOFF2 files, and the page names them as such (decision E17).
func TestMediaConversion(t *testing.T) {
	dir, doc, m := convertMedia(t, pptxgen.Basic())
	pictures := 0
	for _, tg := range doc.Tags {
		if tg.Name != "image" {
			continue
		}
		href := tg.Value("href")
		if strings.HasSuffix(href, ".png") {
			t.Errorf("a PNG picture was not converted: %s", href)
		}
		if strings.HasSuffix(href, ".webp") {
			pictures++
			if b := asset(t, dir, href); !bytes.HasPrefix(b, []byte("RIFF")) || string(b[8:12]) != "WEBP" {
				t.Errorf("%s is no WebP file", href)
			}
		}
	}
	if pictures == 0 {
		t.Error("the deck has no WebP picture")
	}
	fontFiles := 0
	for _, tg := range doc.Tags {
		if tg.Name != "style" {
			continue
		}
		css := string(doc.Content(tg))
		if strings.Contains(css, `format("truetype")`) {
			t.Errorf("a font is still declared as TrueType: %s", css)
		}
		for _, part := range strings.Split(css, `url("`)[1:] {
			ref := part[:strings.Index(part, `"`)]
			if !strings.HasSuffix(ref, ".woff2") || !strings.Contains(part, `format("woff2")`) {
				t.Errorf("font source %s is not declared as WOFF2", ref)
				continue
			}
			fontFiles++
			if b := asset(t, dir, ref); !bytes.HasPrefix(b, []byte("wOF2")) {
				t.Errorf("%s is no WOFF2 file", ref)
			}
		}
	}
	if fontFiles == 0 {
		t.Error("the deck has no font file")
	}
	if s := m.Stats(); s.Pictures != pictures || s.Fonts != fontFiles || s.After >= s.Before {
		t.Errorf("stats %+v, want %d pictures, %d fonts and fewer bytes after than before", s, pictures, fontFiles)
	}
}

// TestMediaAnimatedPicture: a GIF that became an animated WebP picture keeps the mark by which
// the player starts it again when its slide is entered.
func TestMediaAnimatedPicture(t *testing.T) {
	dir, doc, _ := convertMedia(t, pptxgen.GIFs())
	marked := 0
	for _, tg := range doc.Tags {
		if tg.Name != "image" {
			continue
		}
		if _, ok := tg.Get("data-df-pptx-gif"); !ok {
			continue
		}
		marked++
		href := tg.Value("href")
		if !strings.HasSuffix(href, ".webp") {
			t.Errorf("the animated picture is %s, want a WebP file", href)
			continue
		}
		if b := asset(t, dir, href); !bytes.Contains(b[:64], []byte("VP8X")) || !bytes.Contains(b, []byte("ANMF")) {
			t.Errorf("%s is no animated WebP file", href)
		}
	}
	if marked != 1 {
		t.Errorf("%d pictures are marked as animated, want 1", marked)
	}
}

// TestMediaHostileDeck: the deck with hostile content converts with the conversion of pictures
// and fonts switched on, too.
func TestMediaHostileDeck(t *testing.T) {
	_, doc, _ := convertMedia(t, pptxgen.Hostile())
	if len(doc.Tags) == 0 {
		t.Fatal("no deck was written")
	}
}

// wmvClip makes a short Windows Media video with sound with the ffmpeg of the machine.
func wmvClip(t *testing.T) []byte {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	out := filepath.Join(t.TempDir(), "clip.wmv")
	cmd := exec.Command(ff, "-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "wmv2", "-c:a", "wmav2", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot make the test clip: %v %s", err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// videoDeck has a sound that starts with its slide and a Windows Media video.
func videoDeck(wmv []byte) *pptxgen.Deck {
	d := pptxgen.New("Video and sound")
	s := d.Add(pptxgen.LayoutBlank)
	id := s.MediaShape(d, "audio", "Tone", pptxgen.Box{X: 600, Y: 300, W: 80, H: 80}, "wav", "audio/wav", pptxgen.WAV(2), pptxgen.TestPNG(80, 80))
	s.Timing = pptxgen.AutoPlay(id, 2000)
	s = d.Add(pptxgen.LayoutBlank)
	s.MediaShape(d, "video", "Old video", pptxgen.Box{X: 340, Y: 200, W: 600, H: 340}, "wmv", "video/x-ms-wmv", wmv, pptxgen.TestPNG(600, 340))
	return d
}

// mediaSources returns the src of the video and of the audio element of a deck.
func mediaSources(doc *htmlscan.Doc) (video, audio string) {
	for _, tg := range doc.Tags {
		switch tg.Name {
		case "video":
			video = tg.Value("src")
		case "audio":
			audio = tg.Value("src")
		}
	}
	return
}

// TestVideoAndSound: with ffmpeg installed, a Windows Media video becomes an MP4 file that the
// slide plays, and the PCM sound becomes FLAC (decision E18). Without ffmpeg the deck is what
// it was before: the video is left out with a warning and the sound stays a WAV file.
func TestVideoAndSound(t *testing.T) {
	wmv := wmvClip(t)
	dir, doc, m := convertMedia(t, videoDeck(wmv))
	if !m.FFmpeg() {
		t.Skip("ffmpeg and ffprobe are not both installed")
	}
	video, audio := mediaSources(doc)
	if !strings.HasSuffix(video, ".mp4") || !strings.HasSuffix(audio, ".flac") {
		t.Fatalf("video %q, audio %q; want an .mp4 and a .flac file", video, audio)
	}
	if b := asset(t, dir, video); len(b) < 12 || string(b[4:8]) != "ftyp" {
		t.Errorf("the video file does not begin like an MP4 file")
	}
	if b := asset(t, dir, audio); !bytes.HasPrefix(b, []byte("fLaC")) {
		t.Errorf("the sound file does not begin like a FLAC file")
	}
	if st := m.Stats(); st.Videos != 1 || st.Sounds != 1 {
		t.Errorf("stats %+v, want one video and one sound", st)
	}

	t.Setenv(media.NoFFmpegEnv, "1")
	_, doc, m = convertMedia(t, videoDeck(wmv))
	video, audio = mediaSources(doc)
	if video != "" || !strings.HasSuffix(audio, ".wav") {
		t.Errorf("without ffmpeg: video %q, audio %q; want no video element and the .wav file", video, audio)
	}
	if st := m.Stats(); st.Videos != 0 || st.Sounds != 0 {
		t.Errorf("without ffmpeg: stats %+v", st)
	}
}
