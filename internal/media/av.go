package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Video and sound are converted with the ffmpeg that is installed on the machine (decision E18).
// deckfold brings no encoder for them: an embedded ffmpeg (WebAssembly, one core, no assembler)
// was measured at 7 s for one frame of 1080p video, see docs/spikes/S9-video-and-sound.md.
// Without ffmpeg, or without the encoder a file needs, the file stays as it is.
//
// ffmpeg reads data from a presentation here, outside any sandbox. So a file goes to ffprobe or
// ffmpeg only when deckfold has told its container from the first bytes itself; the input format
// is always given, the protocols are limited to "file", and the file is a copy with a neutral
// name in a folder of its own. A playlist or a concat script, which names other files, is never
// read as one that way.

// NoFFmpegEnv switches the search for ffmpeg off, so that an output does not depend on what is
// installed (tests, comparisons).
const NoFFmpegEnv = "DECKFOLD_NO_FFMPEG"

// stallTimeout is how long ffmpeg may run without getting further before it is stopped.
var stallTimeout = 5 * time.Minute

type avTool struct {
	ffmpeg, ffprobe string
	encoders        map[string]bool
	fpsMode         bool          // -fps_mode exists (ffmpeg 5.1 and later); before it was -vsync
	sem             chan struct{} // ffmpeg uses all cores itself: one conversion at a time
}

// findAV looks for ffmpeg and ffprobe on the PATH and asks ffmpeg for its encoders; nil when
// one of the two is missing or does not answer.
func findAV() *avTool {
	if os.Getenv(NoFFmpegEnv) == "1" {
		return nil
	}
	ff, err1 := exec.LookPath("ffmpeg")
	fp, err2 := exec.LookPath("ffprobe")
	if err1 != nil || err2 != nil {
		return nil
	}
	t := &avTool{ffmpeg: ff, ffprobe: fp, encoders: map[string]bool{}, sem: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ff, "-hide_banner", "-v", "error", "-encoders").Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && len(f[0]) == 6 {
			t.encoders[f[1]] = true
		}
	}
	help, _ := exec.CommandContext(ctx, ff, "-hide_banner", "-h", "full").Output()
	t.fpsMode = bytes.Contains(help, []byte("-fps_mode"))
	return t
}

// avFormat tells the container of a video or sound file from its first bytes, as the name of
// the ffmpeg demuxer, or "" for anything else. Formats that every browser plays as they are and
// that need no look inside (MP3, FLAC, Ogg) are not listed: they stay untouched.
func avFormat(d []byte) string {
	if len(d) < 16 {
		return ""
	}
	switch {
	case string(d[4:8]) == "ftyp":
		return "mov"
	case string(d[4:8]) == "moov", string(d[4:8]) == "mdat", string(d[4:8]) == "wide", string(d[4:8]) == "free", string(d[4:8]) == "skip", string(d[4:8]) == "pnot":
		// QuickTime files from before the ftyp box; the size has to fit, or any file with
		// these letters at this place would count
		if n := binary.BigEndian.Uint32(d); n >= 8 && int64(n) <= int64(len(d)) {
			return "mov"
		}
	case bytes.HasPrefix(d, []byte{0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11, 0xa6, 0xd9, 0x00, 0xaa, 0x00, 0x62, 0xce, 0x6c}):
		return "asf"
	case string(d[:4]) == "RIFF" && string(d[8:12]) == "AVI ":
		return "avi"
	case string(d[:4]) == "RIFF" && string(d[8:12]) == "WAVE":
		return "wav"
	case string(d[:4]) == "FORM" && (string(d[8:12]) == "AIFF" || string(d[8:12]) == "AIFC"):
		return "aiff"
	case bytes.HasPrefix(d, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "matroska"
	case bytes.HasPrefix(d, []byte{0, 0, 1, 0xba}):
		return "mpeg"
	case len(d) >= 377 && d[0] == 0x47 && d[188] == 0x47 && d[376] == 0x47:
		return "mpegts"
	}
	return ""
}

// probe is what ffprobe says about a file.
type probe struct {
	Streams []probeStream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

type probeStream struct {
	CodecType      string `json:"codec_type"`
	CodecName      string `json:"codec_name"`
	Profile        string `json:"profile"`
	PixFmt         string `json:"pix_fmt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Channels       int    `json:"channels"`
	ColorRange     string `json:"color_range"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	FieldOrder     string `json:"field_order"`
	AvgFrameRate   string `json:"avg_frame_rate"`
	Packets        string `json:"nb_read_packets"`
	Disposition    struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

func (p *probe) first(kind string) *probeStream {
	for i := range p.Streams {
		s := &p.Streams[i]
		if s.CodecType == kind && s.Disposition.AttachedPic == 0 {
			return s
		}
	}
	return nil
}

func (p *probe) duration() float64 {
	d, _ := strconv.ParseFloat(p.Format.Duration, 64)
	return d
}

func (s *probeStream) packets() int {
	n, _ := strconv.Atoi(s.Packets)
	return n
}

func (s *probeStream) fps() float64 {
	a, b, ok := strings.Cut(s.AvgFrameRate, "/")
	x, _ := strconv.ParseFloat(a, 64)
	y, _ := strconv.ParseFloat(b, 64)
	if !ok || y == 0 || x == 0 {
		return 25
	}
	return x / y
}

// run starts ffmpeg or ffprobe in dir with an almost empty environment and returns its standard
// output. With watch it reads the progress ffmpeg reports and stops a run that does not get
// further for stallTimeout.
func (t *avTool) run(ctx context.Context, dir, tool string, watch bool, args ...string) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "TMPDIR=" + dir, "AV_LOG_FORCE_NOCOLOR=1"}
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if !watch {
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, lastLine(errOut.String()))
		}
		return out, nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var mu sync.Mutex
	lastAt := time.Now()
	stalled := false
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(stallTimeout / 10)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				mu.Lock()
				if time.Since(lastAt) > stallTimeout {
					stalled = true
					cancel()
				}
				mu.Unlock()
			}
		}
	}()
	// ffmpeg repeats its progress block also when nothing moved, so only a new position or a
	// larger output counts as progress
	seen := map[string]string{}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), "=")
		if !ok || (key != "out_time_us" && key != "total_size" && key != "frame") {
			continue
		}
		if seen[key] != val {
			seen[key] = val
			mu.Lock()
			lastAt = time.Now()
			mu.Unlock()
		}
	}
	err = cmd.Wait()
	close(done)
	mu.Lock()
	defer mu.Unlock()
	if stalled {
		return nil, errStalled
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, lastLine(errOut.String()))
	}
	return nil, nil
}

var errStalled = errors.New("ffmpeg did not get further and was stopped")

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// input is how every call names the source: the format deckfold found, files only.
func input(format, name string) []string {
	return []string{"-protocol_whitelist", "file", "-f", format, "-i", name}
}

func (t *avTool) probe(ctx context.Context, dir, format, name string) (*probe, error) {
	args := append([]string{"-v", "error", "-count_packets", "-show_format", "-show_streams", "-of", "json"}, input(format, name)[:4]...)
	out, err := t.run(ctx, dir, t.ffprobe, false, append(args, name)...)
	if err != nil {
		return nil, err
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (t *avTool) ffmpegArgs(format, name string, rest ...string) []string {
	args := append([]string{"-nostdin", "-hide_banner", "-v", "error", "-nostats", "-progress", "pipe:1", "-y"}, input(format, name)...)
	return append(args, rest...)
}

// digest returns the MD5 sum ffmpeg computes over what the given arguments write, for
// comparing two files: the same packets, or the same decoded sound.
func (t *avTool) digest(ctx context.Context, dir, format, name string, rest ...string) string {
	args := append([]string{"-nostdin", "-hide_banner", "-v", "error", "-nostats"}, input(format, name)...)
	args = append(args, rest...)
	out, err := t.run(ctx, dir, t.ffmpeg, false, append(args, "-f", "md5", "-")...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// playsEverywhere lists the sound codecs every current browser plays in the container they
// usually come in.
var playsEverywhere = map[string]bool{"mp3": true, "aac": true, "flac": true, "opus": true, "vorbis": true}

// convert returns what to embed for a video or sound file of the given container.
func (t *avTool) convert(ctx context.Context, data []byte, format string) (res Result) {
	t.sem <- struct{}{}
	defer func() { <-t.sem }()
	dir, err := os.MkdirTemp("", "deckfold-av-")
	if err != nil {
		return Result{}
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "in.bin"), data, 0o600); err != nil {
		return Result{}
	}
	p, err := t.probe(ctx, dir, format, "in.bin")
	if err != nil {
		return Result{} // not what its first bytes promise: it stays, like any unreadable asset
	}
	c := &avJob{t: t, ctx: ctx, dir: dir, format: format, before: len(data), src: p}
	v, a := p.first("video"), p.first("audio")
	switch {
	case v != nil:
		return c.video(v, a)
	case a != nil:
		return c.sound(a)
	}
	return Result{}
}

type avJob struct {
	t      *avTool
	ctx    context.Context
	dir    string
	format string
	before int
	src    *probe
}

// finish reads the output file and checks it against what the conversion was to give.
func (c *avJob) finish(name, format, mime, ext string, check func(out *probe) bool) Result {
	out, err := c.t.probe(c.ctx, c.dir, format, name)
	if err != nil || !check(out) {
		return Result{}
	}
	data, err := os.ReadFile(filepath.Join(c.dir, name))
	if err != nil || len(data) == 0 {
		return Result{}
	}
	return Result{Data: data, Mime: mime, Ext: ext, Changed: true}
}

func (c *avJob) sound(a *probeStream) Result {
	codec := a.CodecName
	switch {
	case c.format == "wav" && (codec == "pcm_u8" || codec == "pcm_s16le" || codec == "pcm_s24le"):
		return c.flac(false) // lossless, when it is smaller
	case c.format == "wav" && strings.HasPrefix(codec, "pcm_"):
		return Result{} // floating point, 32 bits, mu-law, A-law: every browser plays these
	case c.format == "wav" && strings.HasPrefix(codec, "adpcm_"):
		return c.flac(true) // Chrome does not play ADPCM in a WAV file
	case c.format == "aiff" && (codec == "pcm_s8" || codec == "pcm_s16be" || codec == "pcm_s24be"):
		return c.flac(true) // Chrome does not play AIFF
	case codec == "alac":
		return c.flac(true) // lossless, and only Safari plays it
	case playsEverywhere[codec] && (c.format == "mov" || c.format == "matroska"):
		return Result{}
	}
	// Sound that browsers do not play everywhere and that has no lossless target every
	// browser plays: MP3, or AAC for more than two channels.
	if a.Channels > 2 {
		if !c.t.encoders["aac"] {
			return Result{Note: missingEncoder("aac", "sound")}
		}
		args := c.t.ffmpegArgs(c.format, "in.bin", "-map", "0:a:0", "-vn", "-c:a", "aac", "-b:a", strconv.Itoa(64*a.Channels)+"k", "-movflags", "+faststart", "-f", "mp4", "out.m4a")
		if _, err := c.t.run(c.ctx, c.dir, c.t.ffmpeg, true, args...); err != nil {
			return failed("sound", err)
		}
		return c.finish("out.m4a", "mov", "audio/mp4", "m4a", func(o *probe) bool {
			s := o.first("audio")
			return s != nil && s.CodecName == "aac" && s.Channels == a.Channels && c.sameLength(o, 0.1)
		})
	}
	if !c.t.encoders["libmp3lame"] {
		return Result{Note: missingEncoder("libmp3lame", "sound")}
	}
	args := c.t.ffmpegArgs(c.format, "in.bin", "-map", "0:a:0", "-vn", "-c:a", "libmp3lame", "-q:a", "0", "-f", "mp3", "out.mp3")
	if _, err := c.t.run(c.ctx, c.dir, c.t.ffmpeg, true, args...); err != nil {
		return failed("sound", err)
	}
	return c.finish("out.mp3", "mp3", "audio/mpeg", "mp3", func(o *probe) bool {
		s := o.first("audio")
		return s != nil && s.CodecName == "mp3" && s.Channels == a.Channels && c.sameLength(o, 0.1)
	})
}

// flac writes the sound as FLAC and keeps it when both files decode to the same samples and,
// unless always is set, the FLAC file is smaller.
func (c *avJob) flac(always bool) Result {
	if !c.t.encoders["flac"] {
		if always {
			return Result{Note: missingEncoder("flac", "sound")}
		}
		return Result{}
	}
	args := c.t.ffmpegArgs(c.format, "in.bin", "-map", "0:a:0", "-vn", "-c:a", "flac", "-compression_level", "12", "-f", "flac", "out.flac")
	if _, err := c.t.run(c.ctx, c.dir, c.t.ffmpeg, true, args...); err != nil {
		return failed("sound", err)
	}
	pcm := []string{"-map", "0:a:0", "-vn", "-c:a", "pcm_s32le"}
	want := c.t.digest(c.ctx, c.dir, c.format, "in.bin", pcm...)
	got := c.t.digest(c.ctx, c.dir, "flac", "out.flac", pcm...)
	if want == "" || want != got {
		return Result{}
	}
	r := c.finish("out.flac", "flac", "audio/flac", "flac", func(o *probe) bool {
		s := o.first("audio")
		return s != nil && s.CodecName == "flac"
	})
	if !always && len(r.Data) >= c.before {
		return Result{}
	}
	return r
}

func (c *avJob) sameLength(out *probe, tolerance float64) bool {
	return math.Abs(out.duration()-c.src.duration()) <= tolerance
}

func (c *avJob) video(v, a *probeStream) Result {
	switch {
	case v.ColorTransfer == "smpte2084" || v.ColorTransfer == "arib-std-b67":
		return Result{Note: "a video with HDR is embedded as it is: deckfold does not convert HDR, and not every browser shows it"}
	case strings.HasPrefix(v.PixFmt, "yuva") || strings.Contains(v.PixFmt, "rgba") || strings.Contains(v.PixFmt, "bgra") || strings.Contains(v.PixFmt, "argb") || strings.Contains(v.PixFmt, "abgr"):
		return Result{Note: "a video with transparency is embedded as it is: H.264 has no transparency"}
	case v.FieldOrder != "" && v.FieldOrder != "progressive" && v.FieldOrder != "unknown":
		return Result{Note: "an interlaced video is embedded as it is: deckfold does not deinterlace"}
	}
	plain := v.CodecName == "h264" && (v.PixFmt == "yuv420p" || v.PixFmt == "yuvj420p")
	soundOK := a == nil || a.CodecName == "aac" || a.CodecName == "mp3"
	if plain && soundOK && c.format == "mov" {
		return Result{} // what every browser plays
	}
	sound := []string{"-an"}
	if a != nil {
		sound = []string{"-map", "0:a:0", "-c:a", "copy"}
		if !soundOK {
			if !c.t.encoders["aac"] {
				return Result{Note: missingEncoder("aac", "video")}
			}
			sound = []string{"-map", "0:a:0", "-c:a", "aac", "-b:a", strconv.Itoa(64*max(a.Channels, 1)) + "k"}
		}
	}
	if plain {
		// the picture is fine, the container or the sound is not: pack it again
		args := append([]string{"-map", "0:v:0", "-c:v", "copy"}, sound...)
		args = append(args, "-movflags", "+faststart", "-f", "mp4", "out.mp4")
		if _, err := c.t.run(c.ctx, c.dir, c.t.ffmpeg, true, c.t.ffmpegArgs(c.format, "in.bin", args...)...); err == nil {
			packets := []string{"-map", "0:v:0", "-c", "copy"}
			want := c.t.digest(c.ctx, c.dir, c.format, "in.bin", packets...)
			if r := c.finish("out.mp4", "mov", "video/mp4", "mp4", func(o *probe) bool {
				return want != "" && want == c.t.digest(c.ctx, c.dir, "mov", "out.mp4", packets...)
			}); r.Changed {
				return r
			}
		}
		// some AVI and MPEG files cannot be packed again without new time stamps: encode
	}
	if !c.t.encoders["libx264"] {
		return Result{Note: missingEncoder("libx264", "video")}
	}
	args := []string{"-map", "0:v:0", "-c:v", "libx264", "-profile:v", "high", "-pix_fmt", "yuv420p", "-preset", "veryslow", "-crf", "18"}
	// keep every frame at its time, also with a variable frame rate
	if c.t.fpsMode {
		args = append(args, "-fps_mode", "passthrough")
	} else {
		args = append(args, "-vsync", "passthrough")
	}
	// say the colours of the source again; ffmpeg does not always carry them over
	for flag, val := range map[string]string{"-colorspace": v.ColorSpace, "-color_primaries": v.ColorPrimaries, "-color_trc": v.ColorTransfer, "-color_range": v.ColorRange} {
		if val != "" && val != "unknown" && val != "reserved" {
			args = append(args, flag, val)
		}
	}
	args = append(args, sound...)
	args = append(args, "-movflags", "+faststart", "-f", "mp4", "out.mp4")
	if _, err := c.t.run(c.ctx, c.dir, c.t.ffmpeg, true, c.t.ffmpegArgs(c.format, "in.bin", args...)...); err != nil {
		return failed("video", err)
	}
	return c.finish("out.mp4", "mov", "video/mp4", "mp4", func(o *probe) bool {
		s := o.first("video")
		if s == nil || s.CodecName != "h264" || s.Profile != "High" || s.PixFmt != "yuv420p" {
			return false
		}
		if s.packets() != v.packets() || !c.sameLength(o, math.Max(0.1, 2/v.fps())) {
			return false
		}
		for want, got := range map[string]string{v.ColorSpace: s.ColorSpace, v.ColorPrimaries: s.ColorPrimaries, v.ColorTransfer: s.ColorTransfer} {
			if want != "" && want != "unknown" && want != "reserved" && got != want {
				return false
			}
		}
		return a == nil || o.first("audio") != nil
	})
}

func missingEncoder(enc, what string) string {
	return fmt.Sprintf("a %s that not every browser plays is embedded as it is: the installed ffmpeg has no %s encoder", what, enc)
}

func failed(what string, err error) Result {
	if errors.Is(err, context.Canceled) {
		return Result{}
	}
	return Result{Note: fmt.Sprintf("a %s that not every browser plays is embedded as it is: ffmpeg could not convert it (%v)", what, err)}
}

// needsFFmpeg tells, without ffmpeg, whether a file of the given container is one that browsers
// do not play everywhere, as far as its first bytes and its header show: the report then says
// that ffmpeg would convert it. A plain MP4 file says nothing here.
func needsFFmpeg(d []byte, format string) bool {
	switch format {
	case "asf", "avi", "aiff", "mpeg", "mpegts":
		return true
	case "mov":
		moov := movBox(d, "moov")
		for _, fourcc := range []string{"hvc1", "hev1", "av01", "apch", "apcn", "apcs", "apco", "ap4h", "alac", "mp4v"} {
			if bytes.Contains(moov, []byte(fourcc)) {
				return true
			}
		}
	}
	return false
}

// movBox returns the content of the first top-level box of the given type, or nil.
func movBox(d []byte, typ string) []byte {
	for pos := 0; pos+8 <= len(d); {
		n := int64(binary.BigEndian.Uint32(d[pos:]))
		head := int64(8)
		if n == 1 && pos+16 <= len(d) {
			n, head = int64(binary.BigEndian.Uint64(d[pos+8:])), 16
		}
		if n == 0 {
			n = int64(len(d) - pos)
		}
		if n < head || int64(pos)+n > int64(len(d)) {
			return nil
		}
		if string(d[pos+4:pos+8]) == typ {
			return d[int64(pos)+head : int64(pos)+n]
		}
		pos += int(n)
	}
	return nil
}
