# Task List (TODO)

## Structure

**Task format:**
```markdown
- [ ] **Title** (Status 🚩) : One-line description
  - [ ] **Subtask 1** : Description
  - [ ] **Subtask 2** : Description
```

**Statuses:**
- `(TODO 🚩)` - Task ready to be started
- `(In progress 🚧)` - Task activated (Workflow step 1)
- `(In review 📋)` - Code done, tests OK, awaiting user review
- `(Paused ⏸️)` - Task blocked or on hold
- `(Done ✅)` - Task validated and committed (Workflow step 7)

**Completed tasks:** Summarize in 1 line with a link to the doc.

**Groups:**
Tasks are organized in sections (### headings) with an emoji and a title.
Sections represent objectives or themes.

---

## Workflow

**7 steps for each task:**

1. **Activation** : Mark `(In progress 🚧)` + propose a plan (Plan mode) ← **Start**
2. **Execution** : Implement technical changes (Build mode)
3. **Validation** : Run tests (`go test ./...`)
4. **Review (BLOCKER)** : Present summary + request user validation
5. **Commit** : Commit with a semantic message (after explicit approval)
6. **Documentation** : Update docs if necessary
7. **Closing** : Mark `(Done ✅)` + 1-line summary with doc link ← **End** (after user validation and commit)

**Rules:**
- ✅ 1 action = 1 validation (after each file modification, test, commit)
- ✅ Test failed → STOP + ask for guidance (don't iterate alone)
- ✅ Build mode required for any code modification
- ✅ Mandatory user review before commit (git diff)
- ✅ One subtask at a time (unless solutions are identical)
- ✅ **Status `(Done ✅)` = validated AND committed** (never mark before review + explicit approval)

**See also:** `skill:todo-workflow` for the full protocol.

---

## 🎯 Objectives

*Active tasks and thematic sections (epic-like).*

### 🚀 Example objective
- [ ] **Task template** (TODO 🚩) : Description
  - [ ] **Subtask 1** : Description
  - [ ] **Subtask 2** : Description

---

## 📥 Backlog

*Ideas, investigations, and identified tasks.*

**External task sources:**
- Jira: https://jira.company.com/projects/PRODA
- GitHub: https://github.com/username/projetperso

- [ ] **Investigation topic** (To investigate 🚩) : Description
- [ ] **Bug template** (To investigate 🚩) :
  - **Observation** : 
  - **Expected** : 
  - **Actual** : 

---

## 🎉 Completed objectives

*Move completed objectives here.*

### 🚀 Example completed objective
- [x] **Task** (Done ✅) : Description
  - [x] **Subtask 1** : Done
  - [x] **Subtask 2** : Done
