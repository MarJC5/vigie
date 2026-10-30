# Vigie in the background

Vigie can keep watch once its window is closed: scan every site at an interval you choose, and
tell you when a scan finds something new and urgent. Each part is off until you turn it on, in
**Settings > General > In the background**.

## Keep Vigie running

With **Keep Vigie running** on, closing the window leaves Vigie running: it leaves the Dock and
keeps its eye in the menu bar (on Linux, the system tray). The eye's menu opens Vigie, scans every
site, says when the next automatic scan is due and how many sites are urgent, and quits. Quit
(⌘Q, or the eye's menu) always quits. With it off, closing the window quits, as before.

## Open at login

Offered once Vigie keeps running: Vigie starts when you log in, with its window closed, in the
menu bar. On macOS 13 and later it is a login item of the system (System Settings > General >
Login Items), where you can also remove it; Vigie reads its state from there and stores none. On
Linux it is an entry in `~/.config/autostart/`. Turning **Keep Vigie running** off removes it.

## Automatic scans

Every 1, 3, 6, 12 or 24 hours, Vigie scans every site, as **Scan all sites** does, with the same
limits per host and your choice for outside lookups. An automatic scan never updates anything, and
never compares the files again (that stays **Check files now**).

- The next scan is due an interval after the latest complete scan of every site, yours or
  automatic, on the clock: the foot of the site list and the eye's menu show its time.
- A computer that slept across several intervals runs one scan when it wakes, not one per interval.
- A scan due while another runs waits for it to end. An automatic scan you stop, or one that does
  not complete, is tried again an interval later.
- Automatic scans run while Vigie runs: its window open, or kept running in the menu bar.

## Notifications

With **Notifications** on (the system asks you once), Vigie tells you when a scan turns a site
urgent, or cannot connect to a host any more. Never for what was already known, never for what got
better: one notification for the sites turned urgent, one for the hosts turned unreachable.

## Scans with Vigie closed

Vigie installs nothing in the system. To scan with Vigie closed, ask your system's scheduler to run
the command line, which exits with 2 when a site is urgent (see [security.md](security.md)). Use it
instead of Vigie's automatic scans, not with them: two programs scanning at once would double the
load on your hosts.

On macOS, a launch agent that scans every hour, saved as
`~/Library/LaunchAgents/io.github.marjc5.vigie.scan.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>io.github.marjc5.vigie.scan</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Applications/Vigie.app/Contents/MacOS/Vigie</string>
    <string>scan</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Minute</key>
    <integer>0</integer>
  </dict>
</dict>
</plist>
```

then `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.marjc5.vigie.scan.plist`.
A calendar interval, unlike `StartInterval`, runs once when the computer wakes if its time passed
during sleep (`man launchd.plist`).

On Linux, a systemd user timer: `~/.config/systemd/user/vigie-scan.service`

```ini
[Service]
Type=oneshot
ExecStart=/usr/local/bin/vigie scan
```

and `~/.config/systemd/user/vigie-scan.timer`

```ini
[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

then `systemctl --user enable --now vigie-scan.timer`.
