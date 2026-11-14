<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# RTMP/RTP Stream Playback

Ability owns the `STREAM_START`, `STREAM_STOP`, and `STREAM_STATUS` commands.
Playback runs in the interactive user session through mpv; this is a receiver
and display client for巡课, not a publisher or recorder.

`STREAM_START` accepts `url` with an absolute `rtmp://` or `rtp://` URL,
optional `fullscreen` boolean, and optional `volume` integer from 0 to 100.
The executable path is configured by `[agent].mpv_path`. If empty, Ability
looks for `mpv.exe` next to itself, then `bin/mpv.exe` next to itself, then
falls back to `mpv.exe` on PATH. Package the complete Windows mpv build and
its required DLLs alongside Ability. The mpv project is GPL-2.0-or-later;
the official installation page links to Windows builds maintained by
third parties, so pin and review the chosen build before distributing it.

The player runs directly without a shell. Only one stream may play at a time.
The status response includes PID, executable path, and the stream URL with
credentials removed. `STREAM_STOP` terminates only the mpv process started by
this Ability instance.

Example payload:

```json
{"url":"rtmp://media.example/live/class-01","fullscreen":true,"volume":60}
```
