# Telegram Setup Guide

[Back to Getting Started](getting-started.md)

This guide walks you through creating a Telegram bot and connecting it to tlive so you can interact with your terminal sessions from Telegram.

## What You'll Need

- A Telegram account
- ~5 minutes

## Step 1: Create a Bot

1. Open Telegram and search for **@BotFather**
2. Send `/newbot`
3. Choose a **display name** (e.g. "My tlive Bot") and a **username** (must end in `bot`, e.g. `my_tlive_bot`)
4. BotFather will reply with a token like `7823456789:AAF-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
5. Copy the **full token** — you'll need it in Step 4

<!-- TODO: screenshot of BotFather conversation -->

> **Tip:** Keep your token secret. Anyone with the token can control your bot.

## Step 2: Get Your Chat ID

Your Chat ID tells tlive where to send messages.

1. Open a chat with your new bot (search for its username and tap **Start**)
2. Send any message (e.g. "hello")
3. Open this URL in your browser (replace `YOUR_TOKEN` with the token from Step 1):
   ```
   https://api.telegram.org/botYOUR_TOKEN/getUpdates
   ```
4. In the JSON response, look for `"chat":{"id":123456789,...}` — that number is your Chat ID
5. For **group chats**, the Chat ID is negative (e.g. `-1001234567890`)

<!-- TODO: screenshot of getUpdates JSON response -->

> **Important:** You must send a message to the bot *before* opening the URL, otherwise the response will be empty.

## Step 3 (Optional): Get User IDs

If you want to restrict who can use the bot, you'll need Telegram User IDs.

1. Search for **@userinfobot** on Telegram and start a chat
2. It will reply with your User ID (e.g. `123456789`)
3. Repeat for each person you want to allow — you'll enter them as comma-separated values

> **Security note:** Setting at least a Chat ID or Allowed User IDs is recommended. Without them, anyone who finds your bot can interact with it.

## Step 4: Configure tlive

You have three options:

**Option A — Interactive setup:**
```bash
tlive setup
```
Select Telegram when prompted, then paste your token and Chat ID.

**Option B — AI-guided setup (recommended):**
```
/tlive setup
```
Run this inside Claude Code for a guided experience.

**Option C — Manual configuration:**

Edit `~/.tlive/config.env`:
```env
TL_ENABLED_CHANNELS=telegram
TL_TG_BOT_TOKEN=your-token
TL_TG_CHAT_ID=your-chat-id
TL_TG_ALLOWED_USERS=user-id-1,user-id-2
```

## Step 5: Verify

1. Start the bridge:
   ```bash
   tlive start
   ```
   Or run `/tlive` in Claude Code.

2. Send a message to your bot in Telegram
3. You should see a response — if so, you're all set!

<!-- TODO: screenshot of successful interaction -->

## Recommended Bot Settings

These are optional but improve the experience. Send each command to **@BotFather**:

| Command | Setting | Why |
|---------|---------|-----|
| `/setprivacy` | Select your bot → `Disable` | Lets the bot read messages in group chats |
| `/setcommands` | See below | Adds a command menu in Telegram |

For `/setcommands`, send this list:
```
new - Start new session
verbose - Set detail level
hooks - Toggle hook approval
```

## Troubleshooting

**Bot not responding**
- Double-check that the token is correct (no extra spaces or missing characters)
- Run `tlive doctor` to check your configuration

**Wrong Chat ID**
- Make sure you sent a message to the bot *first*, then refresh the `getUpdates` URL
- If using a group, make sure the bot has been added to the group

**"Unauthorized" error**
- Your token may have been regenerated in BotFather — go back and copy the latest one
- Each time you reset the token, the old one stops working immediately
