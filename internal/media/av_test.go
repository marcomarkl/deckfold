package media

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// These tests make their clips with the ffmpeg of the machine and are skipped without one. The
// CI installs ffmpeg on the Linux runner, so they run there.

func needAV(t *testing.T) *avTool {
	t.Helper()
	av := findAV()
	if av == nil {
		t.Skip("ffmpeg and ffprobe are not installed")
	}
	return av
}

// clip runs ffmpeg with a test picture and a tone as inputs (0 and 1) and returns the file it
// wrote. It skips the test when this ffmpeg lacks an encoder the clip needs.
func clip(t *testing.T, av *avTool, name string, args ...string) []byte {
	t.Helper()
	for i, a := range args {
		if (a == "-c:v" || a == "-c:a") && i+1 < len(args) && args[i+1] != "copy" && !av.encoders[args[i+1]] {
			t.Skipf("this ffmpeg has no %s encoder", args[i+1])
		}
	}
	out := filepath.Join(t.TempDir(), name)
	all := append([]string{"-v", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1"}, args...)
	if b, err := exec.Command(av.ffmpeg, append(all, out)...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg could not make %s: %v\n%s", name, err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// look asks ffprobe about data, through the same guarded call the converter uses.
func look(t *testing.T, av *avTool, data []byte) *probe {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.bin"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	format := avFormat(data)
	if format == "" {
		switch {
		case bytes.HasPrefix(data, []byte("fLaC")):
			format = "flac"
		default:
			format = "mp3"
		}
	}
	p, err := av.probe(context.Background(), dir, format, "x.bin")
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	return p
}

// samples decodes the first sound stream to 16-bit PCM with ffmpeg, for comparing two files.
func samples(t *testing.T, av *avTool, data []byte, format string) []byte {
	t.Helper()
	in := filepath.Join(t.TempDir(), "x.bin")
	os.WriteFile(in, data, 0o600)
	out, err := exec.Command(av.ffmpeg, "-v", "error", "-nostdin", "-f", format, "-i", in, "-map", "0:a:0", "-f", "s16le", "-c:a", "pcm_s16le", "-").Output()
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return out
}

func TestAVFormat(t *testing.T) {
	av := needAV(t)
	for name, c := range map[string]struct {
		want string
		args []string
	}{
		"a.mp4":  {"mov", []string{"-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv420p"}},
		"a.mov":  {"mov", []string{"-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv420p"}},
		"a.wmv":  {"asf", []string{"-map", "0:v", "-c:v", "wmv2"}},
		"a.avi":  {"avi", []string{"-map", "0:v", "-c:v", "mpeg4"}},
		"a.mkv":  {"matroska", []string{"-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv420p"}},
		"a.mpg":  {"mpeg", []string{"-map", "0:v", "-c:v", "mpeg2video"}},
		"a.ts":   {"mpegts", []string{"-map", "0:v", "-c:v", "mpeg2video"}},
		"a.wav":  {"wav", []string{"-map", "1:a", "-c:a", "pcm_s16le"}},
		"a.aiff": {"aiff", []string{"-map", "1:a", "-c:a", "pcm_s16be"}},
		"a.mp3":  {"", []string{"-map", "1:a", "-c:a", "libmp3lame"}},
		"a.flac": {"", []string{"-map", "1:a", "-c:a", "flac"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := avFormat(clip(t, av, name, c.args...)); got != c.want {
				t.Errorf("container %q, want %q", got, c.want)
			}
		})
	}
	// what names other files is never taken for a video or a sound
	for name, text := range map[string]string{
		"concat script": "ffconcat version 1.0\nfile '/etc/hosts'\n" + strings.Repeat(" ", 400),
		"playlist":      "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\n/etc/hosts\n" + strings.Repeat(" ", 400),
		"text":          strings.Repeat("moov is not at the place of a box type here. ", 20),
	} {
		if got := avFormat([]byte(text)); got != "" {
			t.Errorf("%s is taken for a %s file", name, got)
		}
	}
}

func TestSound(t *testing.T) {
	av := needAV(t)
	o := New()

	// 16-bit PCM in a WAV file becomes FLAC with the same samples
	wav := clip(t, av, "a.wav", "-map", "1:a", "-c:a", "pcm_s16le")
	r := o.Optimize(wav)
	if !r.Changed || r.Ext != "flac" || r.Mime != "audio/flac" || len(r.Data) >= len(wav) {
		t.Fatalf("WAV not converted to FLAC: %+v, %d -> %d bytes", r.Changed, len(wav), len(r.Data))
	}
	if !bytes.Equal(samples(t, av, wav, "wav"), samples(t, av, r.Data, "flac")) {
		t.Error("the FLAC file decodes to other samples than the WAV file")
	}

	// what every browser plays stays: floating point and mu-law in WAV, MP3, AAC in MP4
	for name, args := range map[string][]string{
		"float.wav": {"-map", "1:a", "-c:a", "pcm_f32le"},
		"mulaw.wav": {"-map", "1:a", "-c:a", "pcm_mulaw"},
		"a.mp3":     {"-map", "1:a", "-c:a", "libmp3lame"},
		"a.m4a":     {"-map", "1:a", "-c:a", "aac"},
	} {
		if r := o.Optimize(clip(t, av, name, args...)); r.Changed || r.Note != "" {
			t.Errorf("%s was touched: %+v", name, r.Mime+" "+r.Note)
		}
	}

	// what Chrome does not play becomes FLAC even when that is larger: ADPCM in WAV, AIFF
	for name, c := range map[string]struct {
		format string
		args   []string
	}{
		"adpcm.wav": {"wav", []string{"-map", "1:a", "-c:a", "adpcm_ms"}},
		"a.aiff":    {"aiff", []string{"-map", "1:a", "-c:a", "pcm_s16be"}},
	} {
		src := clip(t, av, name, c.args...)
		r := o.Optimize(src)
		if !r.Changed || r.Ext != "flac" {
			t.Errorf("%s not converted to FLAC", name)
			continue
		}
		if !bytes.Equal(samples(t, av, src, c.format), samples(t, av, r.Data, "flac")) {
			t.Errorf("%s: the FLAC file decodes to other samples", name)
		}
	}

	// sound without a lossless target every browser plays: MP3, and AAC for more than two channels
	wma := o.Optimize(clip(t, av, "a.wma", "-map", "1:a", "-c:a", "wmav2"))
	if !wma.Changed || wma.Ext != "mp3" || wma.Mime != "audio/mpeg" {
		t.Errorf("WMA not converted to MP3: %+v", wma.Mime)
	} else if s := look(t, av, wma.Data).first("audio"); s == nil || s.CodecName != "mp3" {
		t.Errorf("the MP3 file holds %+v", s)
	}
	six := o.Optimize(clip(t, av, "six.aiff", "-map", "1:a", "-ac", "6", "-c:a", "pcm_f32be"))
	if !six.Changed || six.Ext != "m4a" || six.Mime != "audio/mp4" {
		t.Errorf("sound with six channels not converted to AAC: %+v", six.Mime)
	} else if s := look(t, av, six.Data).first("audio"); s == nil || s.CodecName != "aac" || s.Channels != 6 {
		t.Errorf("the AAC file holds %+v", s)
	}
	if st := o.Stats(); st.Sounds != 5 || st.Videos != 0 || st.Pictures != 0 {
		t.Errorf("stats %+v, want 5 sounds", st)
	}
}

func TestVideo(t *testing.T) {
	av := needAV(t)
	o := New()
	h264 := []string{"-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv420p"}
	withSound := func(v []string, a ...string) []string { return append(append([]string{}, v...), a...) }

	// H.264 with AAC in MP4 is what every browser plays: it stays, byte for byte
	plain := clip(t, av, "a.mp4", withSound(h264, "-map", "1:a", "-c:a", "aac")...)
	if r := o.Optimize(plain); r.Changed || r.Note != "" || !bytes.Equal(r.Data, plain) {
		t.Errorf("a plain MP4 file was touched: %+v", r.Mime+" "+r.Note)
	}

	// the same picture in another container is packed again, the pictures untouched
	mkv := clip(t, av, "a.mkv", h264...)
	r := o.Optimize(mkv)
	if !r.Changed || r.Ext != "mp4" || r.Mime != "video/mp4" {
		t.Fatalf("H.264 in Matroska not packed into MP4: %+v", r.Mime+" "+r.Note)
	}
	in, out := look(t, av, mkv).first("video"), look(t, av, r.Data).first("video")
	if out == nil || out.CodecName != "h264" || out.packets() != in.packets() || out.packets() != 10 {
		t.Errorf("packed again: %+v, source %+v", out, in)
	}

	// what browsers do not play everywhere becomes H.264, High profile, 8 bits, 4:2:0, with
	// every frame and the colours of the source
	for name, args := range map[string][]string{
		"hevc.mp4":   withSound([]string{"-map", "0:v", "-c:v", "libx265", "-pix_fmt", "yuv420p10le", "-tag:v", "hvc1", "-x265-params", "log-level=error:colorprim=bt709:transfer=bt709:colormatrix=bt709"}, "-map", "1:a", "-c:a", "aac"),
		"a.wmv":      withSound([]string{"-map", "0:v", "-c:v", "wmv2"}, "-map", "1:a", "-c:a", "wmav2"),
		"a.mpg":      {"-map", "0:v", "-c:v", "mpeg2video"},
		"hi422.mp4":  {"-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv422p10le"},
		"mpeg4.avi":  {"-map", "0:v", "-c:v", "mpeg4"},
		"silent.wmv": {"-map", "0:v", "-c:v", "wmv2"},
	} {
		t.Run(name, func(t *testing.T) {
			src := clip(t, av, name, args...)
			r := o.Optimize(src)
			if !r.Changed || r.Ext != "mp4" || r.Mime != "video/mp4" {
				t.Fatalf("not converted: %+v", r.Mime+" "+r.Note)
			}
			in, out := look(t, av, src), look(t, av, r.Data)
			v := out.first("video")
			if v == nil || v.CodecName != "h264" || v.Profile != "High" || v.PixFmt != "yuv420p" || v.Width != 160 || v.Height != 120 {
				t.Fatalf("the result is %+v", v)
			}
			if v.packets() != in.first("video").packets() {
				t.Errorf("%d frames, the source has %d", v.packets(), in.first("video").packets())
			}
			if name == "hevc.mp4" && (v.ColorSpace != "bt709" || v.ColorPrimaries != "bt709" || v.ColorTransfer != "bt709") {
				t.Errorf("the colours of the source are not named: %+v", v)
			}
			a := out.first("audio")
			if hasSound := in.first("audio") != nil; hasSound != (a != nil) || (a != nil && a.CodecName != "aac") {
				t.Errorf("sound of the result: %+v, the source has sound: %v", a, hasSound)
			}
		})
	}

	// what deckfold does not convert stays, and the report says why
	for name, c := range map[string]struct {
		word string
		args []string
	}{
		"alpha.mov":      {"transparency", []string{"-map", "0:v", "-c:v", "qtrle", "-pix_fmt", "argb"}},
		"interlaced.mpg": {"interlaced", []string{"-map", "0:v", "-c:v", "mpeg2video", "-vf", "setfield=tff", "-flags", "+ilme+ildct"}},
		"hdr.mp4":        {"HDR", []string{"-map", "0:v", "-c:v", "libx265", "-pix_fmt", "yuv420p10le", "-tag:v", "hvc1", "-x265-params", "log-level=error:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc"}},
	} {
		t.Run(name, func(t *testing.T) {
			src := clip(t, av, name, c.args...)
			if r := o.Optimize(src); r.Changed || !strings.Contains(r.Note, c.word) {
				t.Errorf("changed %v, note %q; want it unchanged with a note about %s", r.Changed, r.Note, c.word)
			}
		})
	}
}

// TestHostileMedia: data that only looks like a video does not stop the conversion, stays as it
// is, and nothing it names is read.
func TestHostileMedia(t *testing.T) {
	needAV(t)
	o := New()
	asf := []byte{0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11, 0xa6, 0xd9, 0x00, 0xaa, 0x00, 0x62, 0xce, 0x6c}
	secret := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(secret, []byte("not for the output"), 0o600)
	for name, data := range map[string][]byte{
		"an MP4 header and noise":         append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isomiso2"), bytes.Repeat([]byte{0xab, 0x13, 0x77}, 3000)...),
		"an ASF header and noise":         append(append([]byte{}, asf...), bytes.Repeat([]byte{0x01, 0xfe}, 4000)...),
		"an MP4 header before a playlist": []byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isomiso2#EXTM3U\n#EXTINF:1,\n" + secret + "\n"),
		"a WAV header before a script":    []byte("RIFF\x24\x00\x00\x00WAVEffconcat version 1.0\nfile '" + secret + "'\n"),
		"an empty MP4 box":                []byte("\x00\x00\x00\x08ftyp\x00\x00\x00\x00\x00\x00\x00\x00"),
	} {
		start := time.Now()
		r := o.Optimize(data)
		if r.Changed || !bytes.Equal(r.Data, data) {
			t.Errorf("%s was converted to %s", name, r.Mime)
		}
		if bytes.Contains(r.Data, []byte("not for the output")) {
			t.Errorf("%s: the other file is in the result", name)
		}
		if time.Since(start) > 20*time.Second {
			t.Errorf("%s took %v", name, time.Since(start))
		}
	}
}

// TestWithoutFFmpeg: without ffmpeg every video and sound stays, and the report says so only
// for files that ffmpeg would convert.
func TestWithoutFFmpeg(t *testing.T) {
	av := needAV(t)
	wmv := clip(t, av, "a.wmv", "-map", "0:v", "-c:v", "wmv2")
	mp4 := clip(t, av, "a.mp4", "-map", "0:v", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	wav := clip(t, av, "a.wav", "-map", "1:a", "-c:a", "pcm_s16le")
	t.Setenv(NoFFmpegEnv, "1")
	o := New()
	if o.FFmpeg() {
		t.Fatal("the switch does not hide ffmpeg")
	}
	if r := o.Optimize(wmv); r.Changed || r.Note != noFFmpeg {
		t.Errorf("WMV without ffmpeg: changed %v, note %q", r.Changed, r.Note)
	}
	for name, data := range map[string][]byte{"MP4": mp4, "WAV": wav} {
		if r := o.Optimize(data); r.Changed || r.Note != "" {
			t.Errorf("%s without ffmpeg: changed %v, note %q", name, r.Changed, r.Note)
		}
	}
	if notes := o.Notes(); len(notes) != 1 {
		t.Errorf("notes %q, want the one about ffmpeg once", notes)
	}
}

// TestStalledFFmpeg: a run that reports no progress is stopped.
func TestStalledFFmpeg(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script in place of ffmpeg")
	}
	old := stallTimeout
	stallTimeout = 300 * time.Millisecond
	defer func() { stallTimeout = old }()
	av := &avTool{sem: make(chan struct{}, 1)}
	start := time.Now()
	// the same position again and again is no progress
	_, err := av.run(context.Background(), t.TempDir(), "/bin/sh", true, "-c", "while true; do echo frame=1; echo out_time_us=40000; sleep 0.05; done")
	if err != errStalled || time.Since(start) > 5*time.Second {
		t.Errorf("error %v after %v, want the run stopped as stalled", err, time.Since(start))
	}
	// a run that gets further is left alone
	if _, err := av.run(context.Background(), t.TempDir(), "/bin/sh", true, "-c", "for i in 1 2 3 4 5 6 7 8 9 10; do echo frame=$i; sleep 0.1; done"); err != nil {
		t.Errorf("a run with progress was stopped: %v", err)
	}
}

// TestCancelled: after Ctrl+C a file stays as it is, without a note about a failure.
func TestCancelled(t *testing.T) {
	av := needAV(t)
	wmv := clip(t, av, "a.wmv", "-map", "0:v", "-c:v", "wmv2")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := New()
	o.Context = ctx
	if r := o.Optimize(wmv); r.Changed {
		t.Error("a file was converted after the conversion was cancelled")
	}
}
