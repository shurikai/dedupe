# Dedupe - Photo and Video Organizer

Dedupe is a command-line tool that helps you organize and deduplicate your photo and video collections. It processes files from a source directory, extracts their creation dates from metadata, and organizes them into a structured destination directory by date (year/month/day). It also identifies and handles duplicate files.

## Features

- **Organize photos and videos by date**: Automatically sorts files into year/month/day folders based on creation date metadata
- **Duplicate detection**: Identifies duplicate files using MD5 checksums
- **Metadata extraction**: Extracts creation dates from EXIF data for photos and video metadata
- **Terminal UI**: Real-time progress display with statistics
- **Cross-platform**: Works on Windows, macOS, and Linux

## Installation

### Prerequisites

- Go 1.24 or later

### Building from source

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/dedupe.git
   cd dedupe
   ```

2. Build the application:
   ```
   go build
   ```

## Usage

```
./dedupe <source-dir> <dest-dir>
```

### Arguments

- `<source-dir>`: Directory containing the photos and videos to organize
- `<dest-dir>`: Directory where organized files will be placed

### Example

```
./dedupe ~/Pictures/Unsorted ~/Pictures/Organized
```

### Output Structure

The destination directory will be organized as follows:

```
dest-dir/
├── YYYY/
│   ├── MM/
│   │   └── DD/
│   │       └── filename_checksum.ext
├── duplicates/
│   └── duplicate_files.ext
└── nodata/
    └── files_without_date_metadata.ext
```

- Files with valid creation dates are organized into year/month/day folders
- Duplicate files are moved to the `duplicates` folder
- Files without date metadata are moved to the `nodata` folder

## Logs

Duplicate file information is logged to `duplicates.log` in the current directory.

## Sample Exection in a Wezterm Terminal window

![alt Dedupe UI Screenshot](./assets/dedupe_screenshot.jpg "Screenshot of Dedupe in Terminal")

## UI Controls

- Press `q` or `Ctrl+C` to quit the application

## Dependencies

- [github.com/cajax/yami](https://github.com/cajax/yami) - Video metadata extraction
- [github.com/charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) - UI components
- [github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [github.com/charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [github.com/rwcarlsen/goexif](https://github.com/rwcarlsen/goexif) - EXIF data extraction

## License

[MIT License](LICENSE)