# Wails-Native API Rewrite + UI Perf Fixes

Continuation plan for the desktop app performance work. Phase 1 (backend) is
done and committed; phases 2-5 remain.

## Goal

Replace the stringly-typed `Invoke(method, url, body)` Flask-mirror layer in
`desktop/` with typed Wails bound methods (JSON maps, Go errors), keep the
vanilla JS frontend behind a thin facade, and land the perf fixes. The Python
Flask app (`app.py`, `templates/`, `static/` at repo root) is untouched — the
desktop API intentionally diverges from it.

## Status

- [x] **Phase 1 — Backend typed services** (COMMITTED): services split,
      Invoke/dispatch/routes deleted, mutations return merged save state,
      slow-call logging, tests rewritten, `go test ./...` green.
- [x] **Phase 2 — Regenerate bindings** (COMMITTED): `make build` regenerated
      `frontend/wailsjs/go/bridge/{App,Saves,Editor,Items,Assets,Steam}.{js,d.ts}`
      (wails replaced the stale Bridge.{js,d.ts} itself); `.gitignore` no longer
      ignores `desktop/frontend/wailsjs/`, bindings + runtime wrapper committed.
- [x] **Phase 3 — Frontend facade + call-site migration** (COMMITTED): new
      `dist/static/api.js` (`window.API`, 66 facade methods over the six
      services, [perf] timing + error toasting); all 61 `await api()` app.js
      sites migrated (plan's "75" counted comments too), setup ×3 + waitForBridge,
      wails_api.js Configured; old `api()` deleted; `loadSave` split into
      `applySaveState` (mutations render merged state, single round trip);
      `_reloadMissionsTab(pt, st)`; verified by grep (0 remnants), node
      --check, live launch with temp debug logging (facade + Configured
      paths both fired).
- [x] **Phase 4 — UI perf fixes** (COMMITTED): one delegated
      click/contextmenu/drag* listener set per inventory container (cards
      carry `_item`; `_selectedCard` replaces the per-click
      querySelectorAll scan); rarity pulses are now an opacity-only
      `::after` overlay (`rarityGlow`) with a `prefers-reduced-motion`
      kill switch (elemPulse kept); 3D viewers render on a dirty flag
      (controls "change" — vendored OrbitControls only fires it on real
      camera movement — plus resize) with a every-3rd-frame sweep for the
      rim pulse and `_tickSpecialEffects` inside the render branch only;
      all 13 `<script src>` tags deferred, order kept, inline boot script
      untouched.
- [ ] **Phase 5 — Verification**: feature walk + Performance tab before/after.

## Phase 1 decisions (recorded for continuation)

- Six bound services: `App`, `Saves`, `Editor`, `Items`, `Assets`, `Steam` —
  all wrapping a private `*Bridge` core (never embed it: embedding would
  promote every method into every service's binding surface).
- `deps(filename)` = requireStore + save-name validation (replaces
  `wrapSaveName`); `writeDeps(filename)` adds the game-running guard
  (replaces `wrapGame`).
- `state(filename)` builds the exact payload the old `GET /api/save/{f}`
  returned; `withState(result, filename)` merges it into a mutation result so
  the frontend re-renders from ONE round trip (kills the mutate→refetch
  cycle). Known cost: state is rebuilt via a fresh `ReadSave` (one extra
  decode vs. the theoretical optimum of threading the tree out of the store —
  deliberately skipped to avoid churning ~30 editor.Store signatures).
- `Editor.UpdateCharacter` deliberately does NOT merge state (frontend renders
  the returned char info directly, see BUG-29 comment in app.js).
- Errors are plain `errors.New` — the frontend never branched on HTTP-style
  status codes (verified), and promise rejections surface `err.message`.
- Slow-call log: `slowLog` prints `[perf] <name> took <dur>` to stderr when
  >100ms; wired into `state()`, `SavePreviews`, `AllPartsBatch`,
  `PreviewCodes`, `ImportCodes`, `ExportAll`, `RestoreLoadout`.

## Backend API reference (for the facade + migration)

Return-shape note: "+state" = mutation result map merged with the 5 state
keys (`character`, `inventory`, `missions`, `fast_travel`, `challenges`);
extra result keys (count/added/imported/…) are preserved alongside.

### App → `window.go.bridge.App`
| Method | Replaces | Facade name |
|---|---|---|
| `Detect() map` | GET /api/setup/detect | `API.detect()` |
| `SetupSave(payload)` | POST /api/setup/save | `API.setupSave(cfg)` |
| `DownloadGibbed()` | POST /api/setup/download-gibbed | `API.downloadGibbed()` |
| `Configured() bool` | GET /api/configured | wails_api.js calls directly |
| `ConfigPaths() map` | GET /api/config/paths | `API.configPaths()` |
| `GameStatus() map` | GET /api/game-status | `API.gameStatus()` |
| `SelectFolder(title)` | direct (already) | `API.selectFolder(t)` |
| `OpenPath(path)` | direct (already) | `API.openPath(p)` |

### Saves → `window.go.bridge.Saves`
| Method | Replaces | Facade name |
|---|---|---|
| `ListSaves()` | GET /api/saves | `API.listSaves()` |
| `SavePreviews()` | GET /api/saves/previews | `API.savePreviews()` |
| `LoadSave(filename)` | GET /api/save/{f} | `API.loadSave(f)` |
| `DuplicateSave(filename)` | POST .../duplicate | `API.duplicateSave(f)` |
| `DeleteSave(filename)` | DELETE .../delete | `API.deleteSave(f)` |
| `ListBackups(filename)` | GET .../backups | `API.listBackups(f)` |
| `RestoreBackup(filename, payload{generation})` | POST .../backups/restore | `API.restoreBackup(f, {generation})` |

### Editor → `window.go.bridge.Editor`
| Method | Replaces | Facade name |
|---|---|---|
| `UpdateCharacter(filename, payload)` | POST .../character | `API.updateCharacter(f, body)` — returns char info, NO state |
| `SetSkills(filename, payload{skills})` | POST .../skills | `API.setSkills(f, body)` — +state, keeps count |
| `SetAmmo(filename, payload{ammo})` | POST .../ammo | `API.setAmmo` (frontend-unused, kept) |
| `FillAmmo(filename)` | POST .../ammo/fill | `API.fillAmmo(f)` — +state |
| `Playthrough(filename, payload{action,playthrough})` | POST .../playthrough | `API.playthrough(f, body)` — +state |
| `UnlockAchievements(filename)` | POST .../unlock-achievements | `API.unlockAchievements(f)` — +state, keeps level/missions_completed/challenges_completed/stations_unlocked |
| `SpawnTestWeapons(filename)` | POST .../spawn-test-weapons | kept, frontend-unused |
| `CompleteAllMissions(filename, pt)` | POST .../missions/{pt}/complete-all | `API.completeAllMissions(f, pt)` — +state, keeps count |
| `AddMission(filename, pt, payload{mission,status,level})` | POST .../missions/{pt}/add | `API.addMission(f, pt, body)` — +state |
| `RemoveMission(filename, pt, payload{mission})` | POST .../missions/{pt}/remove | `API.removeMission(f, pt, body)` — +state |
| `AddAllStory(filename, pt, payload{status})` | POST .../missions/{pt}/add-all-story | `API.addAllStory(f, pt, body)` — +state, keeps count |
| `SetActiveMission(filename, pt, payload{mission})` | POST .../missions/{pt}/set-active | `API.setActiveMission(f, pt, body)` — +state |
| `SetMissionStatus(filename, pt, mission, payload{status})` | POST .../missions/{pt}/{*mission} | `API.setMissionStatus(f, pt, mission, {status})` — +state (mission is now a plain arg, no URL encoding) |
| `MissionDB()` | GET /api/missions/db | `API.missionDb()` |
| `UpdateFastTravel(filename, payload{stations})` | POST .../fast-travel | `API.updateFastTravel(f, body)` — +state |
| `UnlockAllFastTravel(filename)` | POST .../fast-travel/unlock-all | `API.unlockAllFastTravel(f)` — +state, keeps added |
| `AllStations()` | GET /api/fast-travel/all-stations | `API.allStations()` |
| `GetChallenges(filename)` | GET .../challenges | `API.getChallenges(f)` |
| `SetChallenge(filename, payload{path,progress,completed})` | POST .../challenges/update | `API.setChallenge` (frontend-unused, kept) |
| `CompleteAllChallenges(filename)` | POST .../challenges/complete-all | `API.completeAllChallenges(f)` — +state, keeps count |
| `ResetAllChallenges(filename)` | POST .../challenges/reset-all | `API.resetAllChallenges(f)` — +state, keeps count |

### Items → `window.go.bridge.Items`
| Method | Replaces | Facade name |
|---|---|---|
| `AddWeapon(filename, payload{balance,level,parts})` | POST .../items/add | `API.addWeapon(f, body)` — +state |
| `AddItem(filename, payload)` | POST .../items/add-item | `API.addItem(f, body)` — +state |
| `ReorderItem(filename, field, payload{from,to})` | POST .../items/{field}/reorder | `API.reorderItem(f, field, body)` — +state |
| `BulkLevel(filename, field, payload{level})` | POST .../items/{field}/bulk-level | `API.bulkLevel(f, field, body)` — +state, keeps count |
| `DeleteItem(filename, field, idx)` | DELETE .../items/{field}/{idx} | `API.deleteItem(f, field, idx)` — +state |
| `SetItemLevel(filename, field, idx, payload{level})` | POST .../{idx}/level | `API.setItemLevel(f, field, idx, body)` — +state |
| `DuplicateItem(filename, field, idx)` | POST .../{idx}/duplicate | `API.duplicateItem(f, field, idx)` — +state |
| `TransferItem(filename, field, idx, payload{to_field})` | POST .../{idx}/transfer | `API.transferItem(f, field, idx, body)` — +state |
| `EditItem(filename, field, idx, payload{parts,game_stage,grade_index})` | POST .../{idx}/edit | `API.editItem(f, field, idx, body)` — +state |
| `PreviewCodes(payload{codes})` | POST /api/preview-codes | `API.previewCodes(body)` |
| `ImportCodes(filename, payload{codes})` | POST .../import | `API.importCodes(f, body)` — +state, keeps imported/errors |
| `ExportCode(filename, field, idx)` | GET .../export/{field}/{idx} | `API.exportCode(f, field, idx)` → {code} |
| `ExportAll(filename)` | GET .../export-all | `API.exportAll(f)` |
| `ListLoadouts()` | GET /api/loadouts | `API.listLoadouts()` |
| `SaveLoadout(payload{filename,name})` | POST /api/loadouts/save | `API.saveLoadout(body)` — keeps weapons/items counts |
| `RestoreLoadout(filename, file, payload{replace})` | POST /api/loadouts/{file}/restore | `API.restoreLoadout(f, file, body)` — +state, keeps imported |
| `DeleteLoadout(file)` | DELETE /api/loadouts/{file} | `API.deleteLoadout(file)` |

### Assets → `window.go.bridge.Assets`
| Method | Replaces | Facade name |
|---|---|---|
| `WeaponTypes()` | GET /api/assets/weapon-types | `API.weaponTypes()` |
| `Balances(weaponType)` | GET /api/assets/balances?type= | `API.balances(type)` |
| `Parts(balance)` | GET /api/assets/parts?balance= | `API.parts(balance)` |
| `ItemCategories()` | GET /api/assets/item-categories | `API.itemCategories()` |
| `ItemBalances(category)` | GET /api/assets/item-balances?category= | `API.itemBalances(cat)` |
| `ItemParts(balance)` | GET /api/assets/item-parts?balance= | `API.itemParts(bp)` |
| `AllPartsForSlot(slot, kind)` | GET /api/assets/all-parts/{slot}?kind= | `API.allPartsForSlot(slot, kind)` |
| `AllPartsBatch(kind, slots)` | GET /api/assets/all-parts-batch | `API.allPartsBatch(kind, slots)` |
| `AllBalances()` | GET /api/assets/all-balances | `API.allBalances()` |
| `Manufacturers()` | GET /api/assets/manufacturers | `API.manufacturers()` |
| `Customizations(class)` | GET /api/assets/customizations/{class} | `API.customizations(class)` |

### Steam → `window.go.bridge.Steam`
`Status()`, `Init()`, `Achievements()`, `Unlock()`, `UnlockAll()`, `Clear()`
(stubs) → `API.steamStatus()` / `API.steamInit()` / `API.steamAchievements()`.

## Phase 3 — Frontend facade design

New `desktop/frontend/dist/static/api.js` (loaded before app.js), defines
`window.API`:

```js
async function call(name, fn) {
    var t0 = performance.now();
    try {
        var res = await fn();
        var dt = performance.now() - t0;
        if (dt > 50) console.warn("[perf] " + name + " " + dt.toFixed(1) + "ms");
        return res;
    } catch (err) {
        var msg = (err && err.message) ? err.message : String(err);
        toast(msg, "error");   // global from app.js
        throw new Error(msg);
    }
}
```

- app.js `api()` wrapper (line ~275) is deleted once all 75 call sites are
  migrated (mapping tables above).
- Split `loadSave` into `applySaveState(data)` (the render block: editor
  visibility, renderCharacter/Equipment/Inventory/Missions/FastTravel/
  Challenges, sidebar active mark) + fetch. Mutations become:
  `const st = await API.<mutation>(...); currentData = st; await applySaveState(st);`
- `_reloadMissionsTab(pt)` gains an optional `st` param: when the caller just
  got merged state, render it instead of re-fetching.
- Direct `window.go.bridge.Bridge.*` references to update: app.js:283
  (deleted with api()), app.js:771 OpenPath → `App.OpenPath`,
  wails_api.js `b.Invoke("GET","/api/configured")` → `App.Configured()`,
  setup/index.html `bridge().Invoke(...)` ×3 → `App.Detect/DownloadGibbed/SetupSave`,
  `waitForBridge` check → `window.go.bridge.App`.

## Phase 4 — UI perf fixes

- **Event delegation** in `renderItemList` (app.js ~2153): keep card creation,
  set `card._item = item`, drop the 7 per-card listeners; attach one
  click/contextmenu/dragstart/dragend/dragover/dragleave/drop set per
  container (`inventory-weapons/items/bank`) once at init; handlers resolve
  `e.target.closest(".item-card")`. Track selected card in a variable instead
  of the per-click `querySelectorAll(".item-card")` scan.
- **CSS** (style.css 977-997, 1023-1076): replace animated `box-shadow`
  keyframes (`legendaryPulse`/`pearlPulse`/`seraphPulse`) with a static
  box-shadow + `::after` overlay animating `opacity`; keep `elemPulse`
  (already opacity-only); add `@media (prefers-reduced-motion: reduce)` kill
  switch.
- **3D** (viewer3d.js ~1578-1617): dirty-flag render loop —
  `controls.addEventListener("change", …)` + resize set `dirty`; render when
  `dirty || frame % 3 === 0` (keeps the rim pulse at ~20fps, idle GPU at 1/3);
  run `_tickSpecialEffects` only inside the render branch.
- **index.html**: add `defer` to the 13 `<script src>` tags (lines 12-20,
  641-643; keep order; the inline boot script on line 7 stays inline).

## Out of scope (follow-ups)

- `Saves.SavePreviews` serial parse-all + first-use Gibbed dump load
  (first-use jank only).
- Typed request structs / bundler / framework adoption.

## Verification checklist (Phase 5)

1. `cd desktop && go test ./...`
2. `make dev` → walk every feature: setup wizard (gibbed progress events),
   save list/load/delete/duplicate/backups, character, skills, missions,
   fast-travel, challenges, item add/edit/move/delete/duplicate, bank,
   loadouts, Steam tab, settings gear.
3. Console: `[perf]` lines appear, no errors; stderr shows slow Go handlers.
4. Performance tab: mutations = single bridge call (no refetch), no long
   tasks >100ms, near-zero GPU with preview idle.
5. Regression: drag-drop reorder, context menus, error toasts (bad filename),
   game-running guard blocks mutations while BL2 runs.
