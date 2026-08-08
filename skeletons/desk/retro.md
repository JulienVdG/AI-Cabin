# Session Retro Process

## Purpose

Continuously improve the AI Agent's behavior based on actual session experience.

## Process

### 1. Analyze the Session

**Review what happened:**
- What went well? (successful patterns, good decisions)
- What went wrong? (violations, misunderstandings, frustrations)
- What was unclear? (ambiguous instructions, missing guidance)
- When did the AI iterate alone vs. ask for validation?

**Reference:** Check `AGENTS.md` ## Workflow Protocol (7 steps) for expected behavior:
- Step 1: Activation (TODO.md + plan)
- Step 2: Execution (implementation)
- Step 3: Validation (tests)
- Step 4: Review (BLOCKER - user approval)
- Step 5: Commit & Iterate
- Step 6: Documentation
- Step 7: Closing (TODO.md update + session title)

**Also check:** `skill:task-closing` for proper closing ritual (Step 7 details)

**Look for patterns:**
- Repeated violations of workflow protocol
- Moments where user had to remind the AI of rules
- Decisions that deviated from the plan
- Test failures that weren't properly discussed
- Skipped validation checkpoints (Step 4, Step 5)

### 2. Ask the User

**Key questions:**
- "What do you think went wrong in this session?"
- "What should I do differently next time?"
- "What worked well that we should keep?"
- "Were there moments where I should have asked for validation?"
- "What was frustrating or unclear?"
- "Were there recurring patterns that could become skills?"

### 2.5. Identify Skills Opportunities

**Ask user:**
- "Were there recurring patterns that could become skills?"
- "Should we create/update a skill for [specific workflow]?"
- "Which skills were useful? Which need updates?"

**Decision criteria:**

| Pattern type | Skill location | Write method |
|--------------|----------------|--------------|
| Global (universal) | `desk/skills/` | In-place (git diff review) |
| Technical (repo-specific) | `.agents/skills/` in project repo | In-place (git diff review) |

**Examples:**
- `skill:todo-workflow`, `skill:validation-checkpoint` → `desk/skills/`
- Repo-specific skills → `.agents/skills/` in that project repo

## Skills Writing

For comprehensive guidelines on writing skills, see [skills-writing.md](skills-writing.md).

**Key points:**
- **Reference, don't duplicate**: Skills should reference docs, not duplicate content
- **Description with tags**: Add keywords in brackets for discovery
- **Structure**: Header → What I do → Quick Reference → Tables → Related
- **Target**: Keep skills concise (< 100 lines)

**Example:**
```yaml
description: Session retrospective following desk/retro.md [feedback, session review, improvement, workflow, validation]
```

**🔁 After adding/updating/removing skills:**

Run the skill sync command for your AI tool, then restart the session.

**Why:** Skills are loaded at startup. Without restarting, new or updated skills won't be available in the next session.

**✅ Checklist for skill changes:**
1. [ ] Create/update/delete skills in `desk/skills/`
2. [ ] Commit the changes
3. [ ] Run skill sync command (tool-specific)
4. [ ] **RESTART SESSION** (Docker or direct)
5. [ ] Verify in next session that skills are discovered

**🚨 Don't skip step 4!** Without restarting, the new skills won't be available in the next session.

---

### 3. Propose Updates

**Three levels of improvements:**

#### 3.1. Agent Instructions (`desk/AGENTS.md` - WRITABLE)

**Direct modification:**
1. Edit `desk/AGENTS.md` directly
   **DO NOT commit** - user reviews with `git diff`
2. **User action**: `git diff` to review changes
3. `git commit` with semantic message (after user approval)
4. **User action**: Run `make setup` in cabin to regenerate `~/.config/opencode/AGENTS.md`

**Why `make setup`?** The cabin copies `desk/AGENTS.md` to `~/.config/opencode/AGENTS.md` at startup.

#### 3.2. Skills (`desk/skills/` - WRITABLE)

**System skills location:**
- Universal workflow skills: `desk/skills/` (e.g., `todo-workflow`, `retro-process`)
- Project-specific skills: `.agents/skills/` in that project repo

**In-place modification:**
1. Edit `desk/skills/*/SKILL.md` directly
   **DO NOT commit** - user reviews with `git diff`
2. **User action**: `git diff` to review changes
3. `git commit` with semantic message (after user approval)
4. **User action**: Restart cabin for new skills to load

**🔁 After skill changes:**
- Skills are loaded at startup. Without restarting, new or updated skills won't be available.
- **Checklist:**
  1. [ ] Create/update/delete skills in `desk/skills/`
  2. [ ] Commit the changes
  3. [ ] **RESTART CABIN** (Docker container)
  4. [ ] Verify in next session that skills are discovered

#### 3.3. Project Documentation (`desk/*.md` - WRITABLE)

**For technical decisions:**
- Check if relevant documentation has been updated for projects worked on this session:
  - `README.md`, `ARCHITECTURE.md` in project repos
  - `desk/{project-name}.md` (e.g., `desk/ai-cabin.md`)
- Propose updates to documentation
- Document patterns discovered
- **DO NOT commit** - user reviews with `git diff`

**Note:** Documentation should ideally be updated before this step (during task closing). The retro is a good time to catch anything that was missed.

**Reference:** See `skill:task-closing` for detailed closing process (Steps 1-7).

## Example Session

```
User: "Let's do feedback on this session"

AI: "Here's my analysis:

✅ **What went well:**
- TDD tests before implementation (good preparation)
- Clean architecture decisions
- Documentation updates (comprehensive)
- Validation checkpoints respected after reminder

❌ **What went wrong:**
- I iterated alone on test failures multiple times
- I used /tmp/ for temporary files (no user access)
- I chained write→test→fix without asking guidance
- I violated "1 action = 1 validation" several times

📝 **Proposed AGENT.md updates:**
1. Add explicit rule: 'When test fails, ask for guidance'
2. Add workspace constraint: 'NEVER use /tmp/'
3. Clarify acceptable pattern: write→test→validate (not write→test→fix→test→...)

🧠 **Skills identified:**
- **Update**: `debug-go` - Add validation checkpoint rules
- **New**: `tdd-snapshots` - DOM exploration tests with YAML

📚 **Architecture updates** (project documentation (README.md, ARCHITECTURE.md, desk/*.md)):
- Document validator package API
- Add TDD workflow section

**What's your perspective on this session?**
- Did I miss anything important?
- What should we prioritize for improvement?"

User: "Yes, and also add the nuance about write→test→validate being acceptable"

AI: "Should I write all these proposals?"

User: "Yes, go ahead"

AI: [writes all proposals in respective files]

User: 
- Git diff AGENTS.md → commits
- Runs `make setup` in cabin to regenerate AGENTS.md
- Git diff skills → commits
- Restarts cabin for new skills to load
- Commits project documentation (README.md, ARCHITECTURE.md, desk/*.md) changes
```

## Benefits

**This ensures:**
- Instructions evolve based on real experience
- Skills capture reusable patterns from sessions
- User has final say on what gets documented/committed
- AI learns from mistakes systematically
- Future sessions benefit from past improvements
- Direct editing with git diff review (no proposal files)

---

## Documentation Index

**Use this index at end of session to identify which docs need updates.**

| Document | Purpose | Update when |
|----------|---------|-------------|
| `desk/DEVELOPMENT.md` | Dev workflow, debugging, tools, Makefile patterns | New tools, debugging lessons, build patterns |
| `desk/TODO.md` | Task tracking | Start/end of tasks, bug discoveries |
| `desk/retro.md` | Session retro process (this file) | Process improvements only |
| Project docs | `README.md`, `ARCHITECTURE.md`, `desk/*.md` | Project-specific decisions |

### End of Session Checklist

Before closing a session, review:
- [ ] Project documentation updated? (README.md, ARCHITECTURE.md, desk/*.md)
- [ ] New tools created? → Add to `desk/DEVELOPMENT.md`
- [ ] Complex features? → Update feature-specific docs
- [ ] Task status changes? → Update `desk/TODO.md`
- [ ] Process improvements? → Update `desk/retro.md` (this file)
- [ ] **AGENTS.md modified?** → Remind user to run `make setup` in cabin
- [ ] **Skills modified?** → Remind user to restart cabin
- [ ] **Session title proposed?** → See `skill:task-closing` Step 6
- [ ] **Workflow Protocol followed?** → Check `AGENTS.md` ## Workflow Protocol (7 steps)
