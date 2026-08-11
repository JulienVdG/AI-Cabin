# Liste des Tâches (TODO)

## Structure

**Format d'une tâche :**
```markdown
- [ ] **Titre** (Statut 🚩) : Description en une ligne
  - [ ] **Sous-tâche 1** : Description
  - [ ] **Sous-tâche 2** : Description
```

**Statuts :**
- `(À faire 🚩)` - Tâche prête à être commencée
- `(En cours 🚧)` - Tâche activée (Workflow étape 1)
- `(En revue 📋)` - Code terminé, tests OK, en attente de review utilisateur
- `(Suspendu ⏸️)` - Tâche bloquée ou en pause
- `(Terminé ✅)` - Tâche validée et commitée (Workflow étape 7)

**Tâches terminées :** Résumer en 1 ligne avec lien vers la doc.

**Groupes :**
Les tâches sont organisées en sections (titres ###) avec emoji et titre.
Les sections représentent des objectifs ou des thèmes.

---

## Workflow

**7 étapes pour chaque tâche :**

1. **Activation** : Marquer `(En cours 🚧)` + proposer un plan (mode Plan) ← **Début**
2. **Exécution** : Implémenter les changements techniques (mode Build)
3. **Validation** : Exécuter les tests (`go test ./...`)
4. **Review (BLOCKER)** : Présenter résumé + demander validation utilisateur
5. **Commit** : Commiter avec message sémantique (après approval explicite)
6. **Documentation** : Mettre à jour les docs si nécessaire
7. **Closing** : Marquer (Terminé ✅) + résumé 1 ligne avec lien doc ← **Fin** (après validation par l'utilisateur et commit)

**Règles :**
- ✅ 1 action = 1 validation (après chaque modification de fichier, test, commit)
- ✅ Test échoué → STOP + demander guidance (ne pas itérer seul)
- ✅ Mode Build requis pour toute modification de code
- ✅ Review utilisateur obligatoire avant commit (git diff)
- ✅ Une sous-tâche à la fois (sauf si solutions identiques)
- ✅ **Statut `(Terminé ✅)` = validé ET commité** (ne jamais marquer avant review + approval explicite)

**Voir aussi :** `skill:todo-workflow` pour le protocole complet.

---

## 🎯 Objectifs

*Tâches actives et sections thématiques (epic-like).*

### 🚀 Exemple d'objectif
- [ ] **Modèle de tâche** (À faire 🚩) : Description
  - [ ] **Sous-tâche 1** : Description
  - [ ] **Sous-tâche 2** : Description

---

## 📥 Backlog

*Idées, investigations, et tâches identifiées.*

**External task sources:**
- Jira: https://jira.company.com/projects/PRODA
- GitHub: https://github.com/username/projetperso

- [ ] **Sujet d'investigation** (À investiguer 🚩) : Description
- [ ] **Modèle de bug** (À investiguer 🚩) :
  - **Observation** : 
  - **Attendu** : 
  - **Actuel** : 

---

## 🎉 Objectifs terminés

*Déplacer ici les objectifs terminés.*

### 🚀 Exemple d'objectif terminé
- [x] **Tâche** (Terminé ✅) : Description
  - [x] **Sous-tâche 1** : Fait
  - [x] **Sous-tâche 2** : Fait
