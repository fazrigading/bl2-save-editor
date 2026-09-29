// Typed facade over the Wails bindings (window.go.bridge.{App,Saves,Editor,Items,Assets,Steam}).
// Every call is wrapped with [perf] timing (>50ms logged to console) and error
// toasting; rejections carry err.message. Loaded before app.js, which provides
// the global toast() used here. Setup page doesn't load app.js, so it calls
// window.go.bridge.App directly instead of this facade.
(function () {
    "use strict";

    async function call(name, fn) {
        var t0 = performance.now();
        try {
            var res = await fn();
            var dt = performance.now() - t0;
            if (dt > 50) console.warn("[perf] " + name + " " + dt.toFixed(1) + "ms");
            return res;
        } catch (err) {
            var msg = (err && err.message) ? err.message : String(err);
            if (typeof toast === "function") toast(msg, "error");
            throw new Error(msg);
        }
    }

    // map: facade name → bound Go method name on window.go.bridge[ns]
    function wire(ns, map) {
        var out = {};
        Object.keys(map).forEach(function (facade) {
            var method = map[facade];
            out[facade] = function () {
                var args = Array.prototype.slice.call(arguments);
                return call(ns + "." + method, function () {
                    var svc = window.go.bridge[ns];
                    return svc[method].apply(svc, args);
                });
            };
        });
        return out;
    }

    window.API = Object.assign(
        wire("App", {
            detect: "Detect",
            setupSave: "SetupSave",
            downloadGibbed: "DownloadGibbed",
            configured: "Configured",
            configPaths: "ConfigPaths",
            gameStatus: "GameStatus",
            selectFolder: "SelectFolder",
            openPath: "OpenPath",
        }),
        wire("Saves", {
            listSaves: "ListSaves",
            savePreviews: "SavePreviews",
            loadSave: "LoadSave",
            duplicateSave: "DuplicateSave",
            deleteSave: "DeleteSave",
            listBackups: "ListBackups",
            restoreBackup: "RestoreBackup",
        }),
        wire("Editor", {
            updateCharacter: "UpdateCharacter",
            setSkills: "SetSkills",
            setAmmo: "SetAmmo",
            fillAmmo: "FillAmmo",
            playthrough: "Playthrough",
            unlockAchievements: "UnlockAchievements",
            spawnTestWeapons: "SpawnTestWeapons",
            completeAllMissions: "CompleteAllMissions",
            addMission: "AddMission",
            removeMission: "RemoveMission",
            addAllStory: "AddAllStory",
            setActiveMission: "SetActiveMission",
            setMissionStatus: "SetMissionStatus",
            missionDb: "MissionDB",
            updateFastTravel: "UpdateFastTravel",
            unlockAllFastTravel: "UnlockAllFastTravel",
            allStations: "AllStations",
            getChallenges: "GetChallenges",
            setChallenge: "SetChallenge",
            completeAllChallenges: "CompleteAllChallenges",
            resetAllChallenges: "ResetAllChallenges",
        }),
        wire("Items", {
            addWeapon: "AddWeapon",
            addItem: "AddItem",
            reorderItem: "ReorderItem",
            bulkLevel: "BulkLevel",
            deleteItem: "DeleteItem",
            setItemLevel: "SetItemLevel",
            duplicateItem: "DuplicateItem",
            transferItem: "TransferItem",
            editItem: "EditItem",
            previewCodes: "PreviewCodes",
            importCodes: "ImportCodes",
            exportCode: "ExportCode",
            exportAll: "ExportAll",
            listLoadouts: "ListLoadouts",
            saveLoadout: "SaveLoadout",
            restoreLoadout: "RestoreLoadout",
            deleteLoadout: "DeleteLoadout",
        }),
        wire("Assets", {
            weaponTypes: "WeaponTypes",
            balances: "Balances",
            parts: "Parts",
            itemCategories: "ItemCategories",
            itemBalances: "ItemBalances",
            itemParts: "ItemParts",
            allPartsForSlot: "AllPartsForSlot",
            allPartsBatch: "AllPartsBatch",
            allBalances: "AllBalances",
            manufacturers: "Manufacturers",
            customizations: "Customizations",
        }),
        wire("Steam", {
            steamStatus: "Status",
            steamInit: "Init",
            steamAchievements: "Achievements",
        })
    );
})();
