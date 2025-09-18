# ToxAV Integration Demo

This example demonstrates complete integration of ToxAV with existing Tox functionality, creating a full-featured Tox client with both messaging and audio/video calling capabilities.

## Overview

The integration demo showcases:

- **Complete Tox Client**: Full messaging and calling functionality
- **Profile Management**: Persistent profile storage and loading
- **Friend Management**: Add friends, track status, messaging
- **Call Integration**: Seamless audio/video calling with messaging
- **Interactive CLI**: Command-line interface for all operations
- **Auto-Response**: Smart message command handling
- **Statistics Tracking**: Comprehensive usage monitoring

## Features

### Messaging Integration
- Send and receive text messages
- Friend request handling
- Message command processing
- Auto-response to special commands
- Message statistics tracking

### Audio/Video Calling
- Initiate audio-only or video calls
- Handle incoming calls automatically
- Call state monitoring and management
- Call duration tracking
- Frame reception statistics

### Profile Management
- Persistent profile storage (`toxav_integration_profile.dat`)
- Automatic profile loading on startup
- Profile backup and recovery
- Friend list persistence

### Interactive Commands
- Complete command-line interface
- Real-time command processing
- Help system and usage guidance
- Statistics and status monitoring

## Usage

### Running the Demo

```bash
cd examples/toxav_integration
go run main.go
```

### First Run Output

```
🚀 ToxAV Integration Demo - Initializing client...
📝 Creating new profile...
✅ Tox ID: 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349ABC123
👤 Name: ToxAV Integration Demo
💬 Status: Integrated Tox client with AV calling
👥 Friends: 0

🎯 ToxAV Integration Demo
========================
This demo shows complete Tox+ToxAV integration.
Type 'help' for commands, Ctrl+C to exit.
🌐 Connected to Tox network

> 
```

### Subsequent Runs

```
📁 Loading existing profile (1024 bytes)
✅ Profile loaded successfully
✅ Tox ID: 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349ABC123
👤 Name: ToxAV Integration Demo
💬 Status: Integrated Tox client with AV calling
👥 Friends: 3
```

## Interactive Commands

### Available Commands

Type `help` at the prompt to see all commands:

```
📋 Available Commands:
  help, h              - Show this help
  friends, f           - List friends
  calls, c             - Show active calls
  stats, s             - Show statistics
  add <tox_id> [msg]   - Add friend
  msg <num> <message>  - Send message
  call <num>           - Audio call
  videocall <num>      - Video call
  hangup <num>         - End call
  save                 - Save profile
  quit, exit, q        - Exit client
```

### Example Usage

#### Adding a Friend

```
> add 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349DEF456 Hello from integration demo!
✅ Friend request sent to 76518406F6A9F221...
```

#### Listing Friends

```
> friends
👥 Friends (2):
  [0] Friend_0 (last seen: 14:32:15)
  [1] Alice Cooper (last seen: 14:30:22)
```

#### Sending Messages

```
> msg 0 Hello there!
📤 Sent: Hello there!
```

#### Making Calls

```
> call 0
📞 Calling Friend_0 (0) - Audio: true, Video: false

> videocall 1
📞 Calling Alice Cooper (1) - Audio: true, Video: true
```

#### Viewing Statistics

```
> stats
📊 Statistics:
  Friends: 2
  Active calls: 1
  Messages sent: 5
  Messages received: 3
  Calls initiated: 2
  Calls received: 1
```

## Message Commands

Friends can send special commands to trigger actions:

### Friend Commands

- **`call`** - Initiates an audio-only call
- **`videocall`** - Initiates an audio+video call
- **`status`** - Requests current client status
- **`help`** - Requests list of available commands
- **`echo <text>`** - Echoes the text back

### Example Interaction

When a friend sends:
```
Friend → "status"
```

The client responds:
```
💬 Message from Alice (0): status
📤 Sent: Status: 2 friends, 1 active calls, messages sent/received: 5/4
```

## Call Management

### Incoming Calls

```
📞 Incoming call from Alice (0) - Audio: true, Video: true
✅ Call answered
📡 Call state with Alice (0): 3
```

### Call Monitoring

```
> calls
📞 Active Calls (1):
  [0] Alice Cooper - 1m23s (Audio: true, Video: true, Frames: 2485)
```

### Ending Calls

```
> hangup 0
📞 Call ended after 2m15s
```

## Profile Persistence

The client automatically saves and loads profiles:

### Automatic Save
- Profile saved on exit
- Manual save with `save` command
- Friend list and settings preserved

### Profile Format
- Binary format compatible with toxcore
- Includes cryptographic keys and friend list
- Secure storage with appropriate file permissions

```
> save
💾 Profile saved (1024 bytes)
```

## Integration Architecture

### Tox Integration
- Uses existing Tox instance for messaging
- Shares friend management with ToxAV
- Unified event handling and callbacks
- Consistent error handling

### ToxAV Integration
- Seamless call initiation from messaging
- Call state synchronization
- Integrated statistics tracking
- Unified user interface

### State Management
- Thread-safe friend and call tracking
- Persistent storage integration
- Real-time status updates
- Graceful error recovery

## Error Handling

The demo includes comprehensive error handling:

### Network Errors
- Bootstrap connection failures
- Message delivery failures
- Call setup failures
- Graceful degradation

### User Input Errors
- Invalid command handling
- Parameter validation
- Clear error messages
- Help system guidance

### Profile Errors
- Profile loading failures
- Save operation failures
- Friend management errors
- Recovery mechanisms

## Performance

### Resource Usage
- Minimal CPU overhead for messaging
- Efficient call state management
- Optimized friend tracking
- Low memory footprint

### Network Usage
- Standard Tox protocol bandwidth
- Optimized call signaling
- Efficient friend discovery
- Bandwidth adaptation

## Configuration

### Default Settings
```go
const (
    defaultAudioBitRate = 64000  // 64 kbps
    defaultVideoBitRate = 500000 // 500 kbps
    saveDataFile        = "toxav_integration_profile.dat"
)
```

### Customization
- Modify bitrates for quality/bandwidth tradeoff
- Change profile storage location
- Adjust auto-response behavior
- Configure call handling preferences

## Extending the Demo

### Adding GUI
Replace command-line interface with graphical interface:

```go
// Replace inputReader() with GUI event handling
// Add visual call controls
// Implement graphical friend list
```

### Advanced Features
Add additional functionality:

```go
// File transfer integration
// Group chat support
// Call recording capabilities
// Contact import/export
```

### Custom Commands
Add new message commands:

```go
// Extend handleMessageCommand() function
// Add command parsing
// Implement new responses
```

## Dependencies

- `github.com/opd-ai/toxcore` - Core Tox functionality
- `github.com/opd-ai/toxcore/av` - ToxAV types and constants

## Related Examples

- `toxav_basic_call/` - Basic audio/video calling
- `toxav_audio_call/` - Audio-only calling with effects
- Standard Tox examples for messaging functionality

## Security Considerations

### Profile Security
- Profile files contain private keys
- Use appropriate file permissions (0600)
- Consider encryption for sensitive deployments
- Regular backup of profile data

### Network Security
- All communications use Tox's built-in encryption
- Friend verification through public keys
- No plaintext transmission of sensitive data
- Resistance to traffic analysis

### Best Practices
- Verify friend public keys manually
- Use secure storage for profile backup
- Regular profile saves to prevent data loss
- Monitor for unusual network activity
