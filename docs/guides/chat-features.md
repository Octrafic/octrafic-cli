# Chat Features

This guide covers the interactive chat interface features in Octrafic, including conversation history, file uploads, commands, and keyboard shortcuts.

## Conversation History

Octrafic automatically saves your conversations with each project, allowing you to resume your work exactly where you left off.

### Automatic Saving

Every chat session is automatically saved with:
- **Full conversation context** — All messages and AI responses
- **Timestamp tracking** — When the conversation was created and last updated
- **Project association** — Each conversation is linked to its project

### Resuming Conversations

When you launch Octrafic without arguments, you can:

1. **Select a project** — Browse your saved projects
2. **Choose a conversation** — View and select from previous conversations
3. **Start fresh** — Create a new conversation for the project

```bash
# Launch interactive mode
octrafic

# Navigate projects with ↑/↓ or j/k
# Press Enter to view conversations
# Select a conversation or create new
```

### Managing Conversations

- **View history** — All conversations are listed with their creation date
- **Continue work** — Resume exactly where you stopped
- **Multiple threads** — Keep separate conversations per project
- **Clear history** — Use `/clear` command to start fresh in current session

## File Uploads

Upload files directly in chat to provide additional context or share API specifications.

### Supported Use Cases

- **API Specifications** — Upload OpenAPI/Swagger files
- **Example Requests** — Share cURL commands or request templates
- **Documentation** — Attach API documentation files
- **Configuration** — Upload environment or config files

### How to Upload

1. **Trigger file picker** — Type `@` followed by a path or press Tab
2. **Navigate** — Use ↑/↓ arrows to browse files and directories
3. **Select** — Press Enter to attach the selected file
4. **Send** — The file is included with your message

```
Example:
> @./specs/api.yaml
[File picker opens]
> [Select file and press Enter]
> Can you analyze this spec and suggest test scenarios?
```

## Chat Commands

Commands help you manage your session and access specific features.

| Command | Description |
|---------|-------------|
| `/clear` | Clear the conversation history in current session |
| `/help` | Show available commands and usage tips |
| `/auth` | Open authentication wizard to configure API credentials |
| `/models` | Select or change the AI model |
| `/info` | Display current project information (URL, spec, auth) |
| `/release-notes` | View latest Octrafic release notes |
| `/logout` | Logout and clear the current session |
| `/exit` | Exit the application |

### Command Examples

```bash
# Check your project configuration
> /info

# Switch to a different AI model
> /models

# Configure authentication
> /auth

# View release notes
> /release-notes
```

## Keyboard Shortcuts

Efficient navigation and control using keyboard shortcuts.

### Navigation

| Shortcut | Action |
|----------|--------|
| `↑` / `↓` | Navigate command history |
| `Page Up` / `Page Down` | Scroll chat viewport |
| `Tab` | Auto-complete file paths |
| `Enter` | Send message |

### Editing

| Shortcut | Action |
|----------|--------|
| `Esc` `Esc` | Clear input (press Esc twice quickly) |
| Standard text editing keys | Navigate and edit text |

### Control

| Shortcut | Action |
|----------|--------|
| `Ctrl+C` | Exit Octrafic |

## Tips & Tricks

### Quick File References

When typing file paths, use Tab for autocomplete:
```
> @./specs/[Tab]
# Shows all files in ./specs/
```

### Command History

Press ↑ to quickly reuse previous commands:
```
> test the /users endpoint
[Later...]
> [Press ↑ to recall and modify]
```

### Clearing Input

Double-tap Esc to quickly clear a long input:
```
> [Long message you want to discard]
> [Esc Esc - input cleared]
```

### Multi-line Context

For complex queries, provide context step by step:
```
> I need to test the authentication flow
> First, explain how the /login endpoint works
> Then suggest test cases for edge cases
```

## Best Practices

### Organizing Conversations

- **Create new conversations** for different testing goals
- **Use descriptive first messages** to identify conversations later
- **Resume conversations** to maintain context and history

### File Uploads

- **Upload specs early** in the conversation for better context
- **Share example responses** to help AI understand expected formats
- **Attach error logs** when troubleshooting issues

### Using Commands

- **Check `/info`** before testing to verify your configuration
- **Use `/clear`** to start fresh without losing saved history
- **Run `/help`** when you need a quick reference

## Related Guides

- [Project Management](./project-management.md) — Managing projects and configurations
- [Authentication](./authentication.md) — Setting up API authentication
- [Providers](./providers.md) — Configuring AI providers and models
