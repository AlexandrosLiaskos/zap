# ⚡ zap

A terminal UI for launching Windows apps. Built with [Go](https://go.dev) and [Charm](https://charm.sh).

## Features

- **Fuzzy search** — type to instantly filter all installed apps
- **Web search** — prefix with `/` to search DuckDuckGo
- **Two sources** — merges Start menu entries and registry install locations
- **Ghost filtering** — hides stale cached entries for uninstalled apps
- **Single binary** — ~3 MB, zero runtime dependencies

## Install

### From source

```
go install github.com/AlexandrosLiaskos/zap@latest
```

### Manual

```
git clone https://github.com/AlexandrosLiaskos/zap
cd zap
go build -ldflags="-s -w" -o zap.exe .
```

Add an alias in your PowerShell profile:

```powershell
Set-Alias -Name "zap" -Value "C:\path\to\zap.exe"
```

## Usage

```
zap
```

- Type to filter apps
- `↑` / `↓` to navigate
- `Enter` to launch
- `/query` to search DuckDuckGo (opens in Chromium)
- `Esc` to quit

## How it works

1. Queries `Get-StartApps` for Start menu entries (UWP + desktop shortcuts)
2. Scans the Windows registry for apps with an `InstallLocation`
3. Merges and deduplicates by display name
4. Filters out ghost entries (uninstalled apps cached by Windows)
5. Launches apps via `explorer.exe shell:AppsFolder\{AppID}` or by opening the install directory

## Requirements

- Windows 10/11
- PowerShell 5.1+ (for `Get-StartApps`)

## See also

- [yeet](https://github.com/AlexandrosLiaskos/yeet) — TUI uninstaller (companion tool)

## License

[MIT](LICENSE)
