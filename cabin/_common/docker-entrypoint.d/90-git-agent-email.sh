#!/bin/bash
# Configure Git agent email from environment variable or default
GIT_EMAIL="${GIT_AGENT_EMAIL:-ai-agent@vdg.name}"
echo "[entrypoint] Configuring git user.email: $GIT_EMAIL"
git config --global user.email "$GIT_EMAIL"
git config --global user.name "AI Agent"
