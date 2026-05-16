#!/bin/bash
# Configure Git agent name and email from environment variables or defaults
GIT_NAME="${GIT_AGENT_NAME:-AI Agent}"
GIT_EMAIL="${GIT_AGENT_EMAIL:-ai-agent@vdg.name}"
echo "[entrypoint] Configuring git user.name: $GIT_NAME, user.email: $GIT_EMAIL"
git config --global user.name "$GIT_NAME"
git config --global user.email "$GIT_EMAIL"
