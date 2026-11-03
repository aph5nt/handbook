"""Claude Code settings shape only. Not run. The real file is JSON, in
.claude/settings.json. It is a dict here so each line can be tagged."""
SETTINGS = {
    "hooks": {
        "PreToolUse": [{                                         # @2
            "matcher": "Bash",                                   # @5 which tool
            "hooks": [{"type": "command",
                       "command": "scripts/block-protected.sh"}],  # @3,6 exit 2 blocks
        }],
        "PostToolUse": [{                                        # @9
            "matcher": "Bash",
            "hooks": [{"type": "command", "command": "scripts/log-tool.sh"}],  # @9
        }],
        "Stop": [{                                               # @10
            "hooks": [{"type": "command", "command": "scripts/require-file.sh"}],  # @10
        }],
    }
}
