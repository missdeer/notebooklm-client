# notebooklm

CLI client for Google NotebookLM — programmatically create notebooks, add sources, generate artifacts (audio podcasts, reports, slides, quizzes, videos, infographics, and more), and download results.

## Installation

```bash
# Install globally
npm install -g @missdeer/notebooklm

# Or run directly with npx
npx @missdeer/notebooklm --help
```

## How It Works

This package uses platform-specific optional dependencies to provide pre-built binaries. When you install `notebooklm`, npm automatically downloads the correct binary for your platform.

### Supported Platforms

| Platform | Architecture | Package |
|----------|--------------|---------|
| macOS    | x64, ARM64   | `@missdeer/notebooklm-darwin-universal` |
| Linux    | x64          | `@missdeer/notebooklm-linux-x64` |
| Linux    | ARM64        | `@missdeer/notebooklm-linux-arm64` |
| Windows  | x64          | `@missdeer/notebooklm-win32-x64` |
| Windows  | ARM64        | `@missdeer/notebooklm-win32-arm64` |

## Quick Start

```bash
# Login (opens browser to authenticate with Google)
notebooklm login

# Generate an audio podcast from a URL
notebooklm audio --url https://example.com/article

# Generate a report from a file
notebooklm report --file ./document.pdf

# See all commands
notebooklm --help
```

## Troubleshooting

### Binary not found

If the platform-specific package failed to install, you can install it manually:

```bash
# For Linux x64
npm install @missdeer/notebooklm-linux-x64

# For macOS
npm install @missdeer/notebooklm-darwin-universal

# For Windows x64
npm install @missdeer/notebooklm-win32-x64
```

## License

GPL-3.0-only

For commercial use, please contact missdeer@gmail.com for licensing options.
