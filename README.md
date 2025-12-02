# Discord Speed Typing Bot POC

A Proof of Concept Discord bot that implements a speed typing game where users must retype sentences within a 10-second time limit.

## Prerequisites

- Go 1.19 or higher
- A Discord Bot Token (get one from [Discord Developer Portal](https://discord.com/developers/applications))

## Installation

1. Clone or download this repository

2. Install dependencies:
```bash
go mod tidy
```

3. Set up your Discord bot token:

   **Option 1: Environment Variable**
   ```bash
   export DISCORD_TOKEN=your_bot_token_here
   ```

   **Option 2: .env File**
   Create a `.env` file in the project root:
   ```
   DISCORD_TOKEN=your_bot_token_here
   ```

## Building

Build the bot:
```bash
go build -o discord-bot main.go
```

## Running

Run the bot:
```bash
./discord-bot
```

Or run directly with Go:
```bash
go run main.go
```

## Bot Setup

Before running the bot, make sure:

1. Your bot is invited to your Discord server with the following permissions:
   - Send Messages
   - Read Message History
   - View Channels

2. Enable "Message Content Intent" in your Discord Application settings:
   - Go to Discord Developer Portal → Your Application → Bot
   - Under "Privileged Gateway Intents", enable "MESSAGE CONTENT INTENT"

## Game Commands

- `!start` - Start a new speed typing game. The bot will send a sentence that you must retype exactly within 10 seconds.

## How It Works

1. Type `!start` in any channel where the bot is present
2. The bot will send a random sentence
3. You have 10 seconds to type it back exactly as shown
4. If you type it correctly: ✅ You win!
5. If you type it incorrectly: ❌ Try again (you can keep trying until time runs out)
6. If time runs out: ⏰ You lose!

## Features

- Thread-safe game state management using `sync.RWMutex`
- Concurrent timeout handling with goroutines
- Multiple users can play simultaneously in different channels
- Graceful shutdown on Ctrl+C
- Bot ignores its own messages

## Project Structure

- `main.go` - Single-file implementation containing all bot logic
- `go.mod` - Go module dependencies
- `README.md` - This file