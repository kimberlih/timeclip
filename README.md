# ⏱️ Timeclip

**Automatic Time Tracking for macOS** - Never forget to log your hours again!

Timeclip is a lightweight, native macOS application that automatically tracks your active work time and seamlessly integrates with popular time tracking services like Magnetic and Clockify. Set it and forget it - Timeclip runs quietly in your menu bar, monitoring your activity and auto-logging time entries when you reach your daily threshold.

[![macOS](https://img.shields.io/badge/macOS-000000?style=for-the-badge&logo=apple&logoColor=F0F0F0)](https://www.apple.com/macos/)
[![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/)
[![SQLite](https://img.shields.io/badge/sqlite-%2307405e.svg?style=for-the-badge&logo=sqlite&logoColor=white)](https://www.sqlite.org/)

## ✨ Features

### 🤖 **Automatic Time Tracking**
- **Smart Detection**: Only tracks time when you're actually working (logged in + laptop lid open + no screensaver)
- **Real-time Monitoring**: Checks your activity every minute with minimal system impact
- **Persistent Storage**: Uses SQLite database to store daily time entries safely
- **Resume Support**: Continues tracking from where you left off when restarting

### 📊 **Menu Bar Integration**
- **Live Progress**: Real-time display of today's tracked time in your menu bar
- **Color-coded Status**: 
  - 🔴 Red: Below daily goal
  - 🟠 Orange: Paused/inactive
  - 🟢 Green: Daily goal reached
- **Interactive Controls**: Pause/resume tracking, view statistics, access settings
- **Smart Tooltips**: Detailed progress information with remaining time and overtime

### 🔗 **API Integration**
- **Magnetic Support**: Full integration with Magnetic time tracking platform
- **Clockify Support**: Complete Clockify API integration
- **Auto-logging**: Automatically creates time entries when you reach your threshold (default: 6 hours)
- **Fallback System**: If preferred API fails, automatically tries backup services
- **Duplicate Prevention**: Prevents double-logging with intelligent state management

### ⚙️ **Intelligent Configuration**
- **GUI Settings**: Beautiful, user-friendly configuration interface (`timeclip-config`)
- **TOML Config**: Human-readable configuration files with validation
- **Flexible Scheduling**: Choose which days to track (default: weekdays only)
- **Customizable Goals**: Set your daily time targets and auto-log thresholds

### 🛡️ **Reliability & Safety**
- **Single Instance Lock**: Prevents multiple instances to avoid database corruption
- **Graceful Shutdown**: Proper cleanup on exit with signal handling
- **Error Recovery**: Robust error handling and automatic recovery from failures
- **Data Integrity**: SQLite database with transaction safety

## 🚀 Quick Start

### Prerequisites
- macOS 10.15+ (Catalina or later)
- Go 1.21+ (for building from source)

### Installation

#### Option 1: Build from Source
```bash
# Clone the repository
git clone https://github.com/yourusername/timeclip.git
cd timeclip

# Build both applications
go build -o timeclip cmd/timeclip/main.go
go build -o timeclip-config cmd/timeclip-config/main.go

# Make executable (if needed)
chmod +x timeclip timeclip-config
```

#### Option 2: Use Build Script
```bash
# Use the provided build script
./scripts/build.sh

# This creates both binaries with proper permissions
```

### First Run

1. **Launch Timeclip**:
   ```bash
   ./timeclip
   ```

2. **Initial Setup**: On first run, Timeclip creates a default configuration file at `~/.timeclip/config.toml` and exits with setup instructions.

3. **Configure APIs**: 
   - **GUI Method**: Run `./timeclip-config` for a user-friendly configuration interface
   - **Manual Method**: Edit `~/.timeclip/config.toml` directly

4. **Start Tracking**: Once configured, run `./timeclip` again - it will start tracking automatically in your menu bar!

## 📋 Configuration

### GUI Configuration (Recommended)

Launch the configuration interface:
```bash
./timeclip-config
```

The GUI provides an intuitive interface with tabs for:
- **General**: Daily goals, auto-log thresholds, tracking days
- **API Settings**: Magnetic and Clockify configuration with secure password fields
- **Interface**: Menu bar and database settings

### Manual Configuration

Timeclip uses a TOML configuration file located at `~/.timeclip/config.toml`:

```toml
[general]
goal_time_hours = 8                    # Daily time goal in hours
auto_log_threshold_hours = 6.0         # Auto-log when reaching this many hours
track_days = ["monday", "tuesday", "wednesday", "thursday", "friday"]
check_interval_seconds = 60            # How often to check system state

[database]
path = "~/.timeclip/timeclip.db"       # SQLite database location

[api]
preferred_provider = "magnetic"         # "magnetic" or "clockify"
retry_attempts = 3                     # Number of retry attempts for API calls
timeout_seconds = 30                   # API request timeout

[api.magnetic]
enabled = true
base_url = "https://app.magnetichq.com/v2/rest/coreAPI"
api_key = "your-magnetic-api-key"
workspace_id = "your-workspace-id"
project_id = "your-project-id"

[api.clockify]
enabled = false
base_url = "https://api.clockify.me/api/v1"
api_key = "your-clockify-api-key"
workspace_id = "your-workspace-id" 
project_id = "your-project-id"

[ui]
show_menu_bar = true                   # Enable menu bar interface
```

## 🔑 API Setup

### Magnetic
1. Log into your [Magnetic account](https://app.magnetichq.com/)
2. Go to **Settings** > **API** and generate an API key
3. Find your workspace and project IDs in the URL when viewing projects
4. Add credentials to configuration (via GUI or manual editing)

### Clockify  
1. Log into your [Clockify account](https://clockify.me/)
2. Go to **Profile Settings** > **API** and generate an API key
3. Find workspace and project IDs in your Clockify dashboard
4. Add credentials to configuration (via GUI or manual editing)

## 💡 How It Works

### System Monitoring
Timeclip uses macOS system APIs to detect:
- **User Session**: Are you logged in?
- **Laptop Lid**: Is the lid open?
- **Screensaver**: Is the screensaver active?

Only when **all conditions are met** (logged in + lid open + no screensaver) does Timeclip count the time as "active work time."

### Auto-logging Process
1. **Continuous Tracking**: Timeclip monitors your activity every minute
2. **Database Storage**: Stores time data locally in SQLite database
3. **Threshold Detection**: When you reach your threshold (default: 6 hours), auto-logging triggers
4. **API Integration**: Creates a time entry in your preferred service (Magnetic/Clockify)
5. **Fallback Support**: If the preferred service fails, tries backup APIs
6. **Duplicate Prevention**: Marks entries as logged to prevent double-logging

### Menu Bar Interface
- **Live Updates**: Shows current day's tracked time (e.g., "⏱ 3.2h")
- **Smart Display**: Automatically formats hours/minutes based on duration
- **Progress Indicators**: Color changes based on goal progress
- **Interactive Menu**: 
  - View detailed statistics
  - Pause/resume tracking
  - Launch configuration GUI
  - Quit application
- **Rich Tooltips**: Hover for detailed progress, remaining time, or overtime information

## 🏗️ Project Structure

```
timeclip/
├── cmd/
│   ├── timeclip/           # Main application
│   └── timeclip-config/    # GUI configuration tool
├── internal/
│   ├── api/               # API integration (Magnetic, Clockify)
│   ├── config/            # Configuration management
│   ├── database/          # SQLite operations
│   ├── instance/          # Single instance locking
│   ├── menubar/           # macOS menu bar interface
│   ├── models/            # Data structures
│   └── tracker/           # Time tracking and system monitoring
├── configs/               # Configuration examples
└── scripts/              # Build and deployment scripts
```

### Key Components

- **System Monitor**: Native macOS integration using CGO for activity detection
- **Activity Detector**: Core time tracking logic with database persistence
- **Timer**: High-level orchestration of tracking with callbacks
- **API Clients**: Robust HTTP clients for Magnetic and Clockify APIs
- **Menu Bar Manager**: Native systray integration with real-time updates
- **Configuration Manager**: TOML-based config with validation and auto-generation
- **Instance Lock**: File-based locking to prevent multiple instances

## 🧪 Development

### Building

```bash
# Build main application
go build -o timeclip cmd/timeclip/main.go

# Build configuration GUI
go build -o timeclip-config cmd/timeclip-config/main.go

# Build both using script
./scripts/build.sh
```

### Testing

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Test specific component
go test ./internal/tracker
go test ./internal/api
```

### Dependencies

- **Core**: Standard Go library with minimal external dependencies
- **GUI**: [Fyne](https://fyne.io/) for cross-platform GUI components
- **Menu Bar**: [getlantern/systray](https://github.com/getlantern/systray) for native macOS menu bar
- **Config**: [pelletier/go-toml](https://github.com/pelletier/go-toml) for TOML parsing
- **Database**: [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) for pure Go SQLite

## 🔧 Troubleshooting

### Common Issues

**Menu bar doesn't appear:**
- Check that `show_menu_bar = true` in config.toml
- Ensure you have menu bar access permissions in System Preferences

**"Another instance is running" error:**
- Only one instance of Timeclip can run at a time to prevent data corruption
- If you're sure no other instance is running, delete `~/.timeclip/timeclip.lock`

**API authentication failures:**
- Verify your API keys are correct in the configuration
- Check that workspace and project IDs are valid
- Ensure API endpoints are accessible from your network

**Time not tracking:**
- Verify your current day is in the `track_days` configuration
- Check that your system meets the activity requirements (logged in + lid open + no screensaver)

### Debug Mode

Run with verbose logging:
```bash
# The application outputs detailed logs to stderr
./timeclip 2> debug.log
```

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. **Fork the Repository**: Create your own fork of Timeclip
2. **Create a Feature Branch**: `git checkout -b feature/amazing-feature`
3. **Make Your Changes**: Implement your feature or bug fix
4. **Add Tests**: Ensure your changes are well-tested
5. **Commit Changes**: `git commit -m 'Add amazing feature'`
6. **Push to Branch**: `git push origin feature/amazing-feature`
7. **Open a Pull Request**: Submit your changes for review

### Development Guidelines

- **Code Style**: Follow standard Go formatting (`gofmt`, `go vet`)
- **Documentation**: Include clear documentation for new features
- **Testing**: Add tests for new functionality
- **Security**: Never commit API keys or sensitive data
- **Compatibility**: Maintain macOS 10.15+ compatibility

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🐛 Issues & Support

- **Bug Reports**: [GitHub Issues](https://github.com/yourusername/timeclip/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/yourusername/timeclip/discussions)
- **Documentation**: [Project Wiki](https://github.com/yourusername/timeclip/wiki)

## 🙏 Acknowledgments

- [Fyne](https://fyne.io/) - Cross-platform GUI toolkit for configuration interface
- [getlantern/systray](https://github.com/getlantern/systray) - System tray integration
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go SQLite driver
- [pelletier/go-toml](https://github.com/pelletier/go-toml) - TOML configuration parsing

---

**Made with ❤️ for developers who forget to track their time**

*Timeclip - Because your time is valuable, and tracking it shouldn't be a chore.*