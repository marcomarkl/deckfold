# S9: Video and sound

Measured on 2026-11-16 on a Mac with twelve cores. The question was how deckfold converts video and sound that browsers do not play everywhere, after pictures and fonts (S8). The plan was an ffmpeg inside the binary, so that nothing has to be installed. The measurements below ended that plan: decision E18 uses the ffmpeg that is installed on the machine.

## An ffmpeg inside the binary

`codeberg.org/gruf/go-ffmpreg` v0.6.22 is ffmpeg n5.1.10 with libx264, built as WebAssembly and run by wazero. It has no threads and no assembler, and its memory ends at 4 GB.

What works:

- The module is ready 1.5 s after the start.
- It has the encoders libx264, flac, libmp3lame and aac and the decoders for HEVC, AV1, WMV (wmv3, vc1), MPEG-2, WMA (wmav2, wmapro) and ADPCM.
- `ffprobe` is the same module under another name.

The time for H.264 with `-crf 18`, on cuts of two HEVC videos of a local collection:

| Source | Preset | Frames | Time | Per frame | Memory of the process |
|---|---|---|---|---|---|
| 1920 x 1080, 8 bits | medium | 30 | 36 s | 1.2 s | 1.2 GB |
| 1920 x 1080, 8 bits | veryslow | 30 | 148 s | 4.9 s | 1.9 GB |
| 1920 x 1080, 8 bits | veryslow | 65 | 452 s | 7.0 s | 2.7 GB |
| 4096 x 2160, 10 bits | medium | 10 | 52 s | | 2.6 GB |
| 4096 x 2160, 10 bits | veryslow | 10 | 103 s | | 3.3 GB |
| 4096 x 2160, 10 bits | veryslow | 55 | 992 s | 18 s | 8.3 GB |

The short runs do not fill the lookahead of the encoder, so the longer ones are the ones to go by. Decoding alone took 2.5 s for 30 frames of the first video and 24 s for 10 frames of the second, most of it for the frames before the cut.

The collection has three HEVC videos: 5373 frames of 1920 x 1080 and 894 and 869 frames of 4096 x 2160. At 7 and 18 s a frame that is about 10 hours for the first and 4.5 and 4.3 hours for the others, 19 hours of processor time together, or a good 10 hours with the three side by side and more than 8 GB of memory for each of the large ones. With the preset "medium" it would still be hours. The 4096 x 2160 video does run through in the 32-bit module.

The module sees only the folder it is given. Tried with a folder mounted as `/work`:

- `-i /etc/hosts`: "No such file or directory".
- A concat script in the folder that names `/etc/hosts`: "Impossible to open '/etc/hosts'".
- A playlist (`.m3u8`) in the folder that names `/etc/hosts`: "Invalid data found".
- Writing to `/outside.mp4`: "No such file or directory", and no file appeared.
- An `https:` address: "Protocol not found". The module is built without network protocols.
- A symbolic link in the folder that points outside it is followed. A folder for the module must therefore hold only files that deckfold wrote itself.

Two more things came up. The module builds x264 from the `master` branch without a fixed commit, so the sources of the embedded binary cannot be named, which a release under the GPL has to do; deckfold would have had to build the module itself, in a container. And with x264 inside, deckfold as a whole would have had to change to the GPL.

## The decision

The maintainer saw these numbers on 2026-11-16 and chose: video and sound are converted with the ffmpeg of the machine, when there is one; otherwise they stay as they are, and the report says so. Converted videos are not kept between runs. deckfold then ships no encoder for video or sound, stays under the MIT licence, and its output for such files depends on the ffmpeg that is installed. A native ffmpeg uses assembler and every core.

## The same videos with the installed ffmpeg

With ffmpeg 9.0.2 from Homebrew, `-crf 18 -preset veryslow`, as deckfold calls it:

| Deck | Videos converted | Time for the deck | Video before | after |
|---|---|---|---|---|
| one | two of 4096 x 2160, 10 bits, 35 s each | 238 s | 65.2 MB | 58.5 MB |
| another | one of 1920 x 1080, 179 s | 337 s | 324 MB | 178 MB |

That is under ten minutes where the embedded module would take about 19 hours. The pictures of both decks came from the cache in these runs. The H.264 videos of the decks stayed as they were. deckfold probed every result: H.264, High profile, 8 bits 4:2:0, the same number of frames and the same length as the source. In the converted decks the three videos played in Chrome and in WebKit (Playwright), at their size and length and without an error; before, Chrome showed these HEVC videos only where the machine can decode them.

## What browsers play of WAV and AIFF

A tone of one second in each encoding, loaded into an `<audio>` element from a file, on 2026-11-16. "Plays" means the element reported `canplaythrough`; "refused" means it reported error 4.

| File | Chrome | WebKit (Playwright) |
|---|---|---|
| WAV, PCM 8, 16, 24, 32 bits | plays | plays |
| WAV, floating point 32 bits | plays | plays |
| WAV, mu-law, A-law | plays | plays |
| WAV, ADPCM (Microsoft, IMA) | refused | plays |
| AIFF, PCM 16 bits | refused | plays |
| FLAC | plays | plays |

Firefox was not measured here; it does not start on this Mac. The rules follow Chrome: ADPCM in a WAV file and AIFF always become FLAC, PCM in a WAV file becomes FLAC when that is smaller, and the other WAV files stay.

## What ffmpeg gets to read

ffmpeg reads data from a presentation, and the installed one runs outside any sandbox. deckfold therefore tells the container of a file from its first bytes itself (MP4 and QuickTime, ASF, AVI, WAV, AIFF, Matroska, MPEG program and transport streams) and gives nothing else to ffprobe or ffmpeg. Every call names that format with `-f` and limits the protocols to `file`. The file is a copy named `in.bin` in a temporary folder of its own, which is also the working folder and the home of the process. A concat script or a playlist is text, has none of these beginnings and never reaches ffmpeg; one that is hidden behind such a beginning is read as the format the beginning promises, fails there and stays as it is. The tests feed such files and check that the file they name is not in the result.

A run that reports no new position for five minutes is stopped, and the file stays as it is with a warning.
