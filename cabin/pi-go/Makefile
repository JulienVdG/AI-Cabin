SHELL := /usr/bin/env bash

# Must match home in container from dockerfile
export CONTAINER_HOME=/home/ai_agent

.DEFAULT_GOAL := all

ifndef V
.SILENT:
endif

.PHONY: all setup setup-dirs docker-build docker-debug-build docker-up docker-down docker-shell docker-logs docker-greyshell

all: docker-up info

.PHONY: guard-%
guard-%:
	@ if [ ! -v $(*) ]; then \
		echo "missing environment variable $(*)"; \
		exit 1; \
	fi


# Generic rule template: copy source to target only if target doesn't exist
# $(1) = target, $(2) = source
define COPY_IF_NOT_EXISTS
$(1): $(2) | setup-dirs
	if [ -e "$$@" ]; then \
		echo "$$@ already exists, not updating"; \
		if [ -n "$(D)" ]; then \
			diff -u "$$<" "$$@"; \
		else \
			echo "compare to $$(realpath $$<) to manually update"; \
		fi; \
	else \
		cp "$$<" "$$@" ;\
	fi
endef

# Generate rules for config files
$(eval $(call COPY_IF_NOT_EXISTS,${AI_CABIN_HOME}/.config/greywall/greywall.json,../_greywall/greywall.json))
$(eval $(call COPY_IF_NOT_EXISTS,${AI_CABIN_HOME}/.config/greywall/learned/workspace.json,../_greywall/profile/workspace.json))
$(eval $(call COPY_IF_NOT_EXISTS,${AI_CABIN_HOME}/.config/greywall/learned/pi.json,../_greywall/profile/pi.json))
$(eval $(call COPY_IF_NOT_EXISTS,${AI_CABIN_HOME}/.pi/agent/settings.json,settings.json))

# models.json with templating (replace __SCW_PROJECT_ID__)
.PHONY: ${AI_CABIN_HOME}/.pi/agent/models.json
${AI_CABIN_HOME}/.pi/agent/models.json: models.json.template guard-SCW_PROJECT_ID | setup-dirs
	sed "s/__SCW_PROJECT_ID__/${SCW_PROJECT_ID}/g" "$<" > "$@"; \

# Phony to force the regeneration of the file if any vars change.
.PHONY: ${AI_CABIN_HOME}/.pi/agent/AGENTS.md guard-AI_CABIN_HOME guard-AI_CABIN_DESK guard-CONTAINER_HOME setup-dirs
${AI_CABIN_HOME}/.pi/agent/AGENTS.md: ${AI_CABIN_DESK}/AGENTS.md
	@# Agent instructions with workspace paths appended
	@cat "${AI_CABIN_DESK}/AGENTS.md" > "$@"
	@echo "" >> "$@"
	@echo "---" >> "$@"
	@echo "" >> "$@"
	@echo "## Workspace Directories (Reference)" >> "$@"
	@echo "" >> "$@"
	@echo "Reference paths for this workspace:" >> "$@"
	@echo "" >> "$@"
	@echo "- **Home**: ${CONTAINER_HOME}" >> "$@"
	@echo "- **Desk**: ${CONTAINER_HOME}/desk/" >> "$@"
	@echo "- **Skills**: ${CONTAINER_HOME}/desk/skills/" >> "$@"
	@echo "- **Workdir**: $${CONTAINER_WORKDIR:-${AI_CABIN_WORKDIR}}" >> "$@"
	@echo "- **Cache**: ${CONTAINER_HOME}/.cache/" >> "$@"
	@echo "" >> "$@"
	@echo "**Note:** Always use absolute paths (e.g., \`${CONTAINER_HOME}/desk/TODO.md\`) not relative paths." >> "$@"

setup-dirs: guard-AI_CABIN_HOME guard-AI_CABIN_DESK guard-AI_CABIN_WORKDIR
	# Agent Data directories (in AI_CABIN_HOME)
	mkdir -p ${AI_CABIN_HOME}/.pi/agent
	mkdir -p ${AI_CABIN_HOME}/.config/greywall/learned
	# Desk directory (in AI_CABIN_DESK)
	mkdir -p ${AI_CABIN_DESK}
	mkdir -p ${AI_CABIN_DESK}/skills
	# Cabin-specific data (in agent_data/)
	mkdir -p agent_data/cache


setup: guard-AI_CABIN_HOME guard-AI_CABIN_DESK guard-AI_CABIN_WORKDIR ${AI_CABIN_HOME}/.config/greywall/greywall.json ${AI_CABIN_HOME}/.config/greywall/learned/workspace.json ${AI_CABIN_HOME}/.config/greywall/learned/pi.json ${AI_CABIN_HOME}/.pi/agent/models.json ${AI_CABIN_HOME}/.pi/agent/settings.json ${AI_CABIN_HOME}/.pi/agent/AGENTS.md
	# Cabin-specific data (in agent_data/)
	touch agent_data/bash_history
	# Used by docker build
	cp ~/.local/bin/greywall . 2>/dev/null || true
	cp ~/.local/share/greyproxy/ca-cert.pem greyproxy-ca.crt 2>/dev/null || true

docker-build: setup
	BUILDKIT_PROGRESS=plain docker compose build

docker-debug-build: setup
	BUILDKIT_PROGRESS=plain docker compose build --no-cache

docker-debug-run: setup
	docker compose run --rm --service-ports agent /bin/bash

# Start everything in background
docker-up: setup
	docker compose up -d

# Stop everything
docker-down:
	docker compose down

# Get a bash shell inside the ALREADY running agent
docker-shell:
	docker compose exec agent bash

# Get a greyshell (greywall sandboxed shell)
docker-greyshell:
	docker compose exec agent greybash

# Continue pi.dev session (greywall sandboxed TUI)
pi:
	docker compose exec agent greypi -c

# Follow logs
docker-logs:
	docker compose logs -f agent

# Restart agent
docker-restart-agent: setup
	docker compose restart agent

up: docker-up info
down: docker-down

# Helpful reminder
info:
	@echo "🛡️ greyproxy (host):            http://localhost:43080"
	@echo "🤖 AI Agent (pi.dev TUI):       Use 'make pi' or './cabin-pi'"

help:
	@echo "Environment variables (must be set before running make):"
	@echo "  AI_CABIN_HOME      : Host home directory (e.g., /home/user/)"
	@echo "  AI_CABIN_DESK      : Shared desk directory (AGENTS.md, skills) (e.g., /home/user/ai-cabin-desk/)"
	@echo "  AI_CABIN_WORKDIR   : Git repositories (e.g., /home/user/workdir/)"
	@echo "  SCW_PROJECT_ID     : Scaleway project ID"
	@echo "  CONTAINER_WORKDIR  : Container workdir (default: AI_CABIN_WORKDIR, advanced)"
	@echo ""
	@echo "Usage:"
	@echo "  make setup             # Setup environment (directories + config)"
	@echo "  make docker-up         # Start cabin"
	@echo "  make docker-shell      # Get shell inside container"
	@echo "  make pi                # pi.dev TUI"
	@echo "  ./cabin-pi             # pi.dev TUI (wrapper from host)"
	@echo "  ./cabin-pi -p \"ask ...\"   # pi.dev CLI with args"
	@echo "  make docker-greyshell  # Greywall sandboxed shell"
	@echo "  make docker-logs       # Follow logs"
	@echo ""
	@echo "Advanced:"
	@echo "  CONTAINER_WORKDIR=/workspace make docker-up  # Remap workdir to /workspace"
