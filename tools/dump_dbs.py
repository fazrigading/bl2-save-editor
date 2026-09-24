#!/usr/bin/env python3
"""Dump hardcoded DBs (missions, stations, achievements) from the Python app
into JSON files embedded by the Go implementation."""
import json
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, ROOT)
sys.path.insert(0, os.path.join(ROOT, "borderlands2-tool"))

import save_io  # noqa: E402
import steam_achievements  # noqa: E402

OUT = os.path.join(ROOT, "desktop", "internal", "editor", "data")


def main():
    os.makedirs(OUT, exist_ok=True)

    missions = [list(m) for m in save_io.MISSION_DB]
    with open(os.path.join(OUT, "missions.json"), "w") as f:
        json.dump(missions, f, indent=1)

    stations = {
        "display_names": save_io.FAST_TRAVEL_STATIONS,
        "base": save_io.ALL_BASE_STATIONS,
        "dlc": save_io.DLC_STATIONS,
    }
    with open(os.path.join(OUT, "stations.json"), "w") as f:
        json.dump(stations, f, indent=1)

    achievements = [list(a) for a in steam_achievements.BL2_ACHIEVEMENTS]
    with open(os.path.join(OUT, "achievements.json"), "w") as f:
        json.dump(achievements, f, indent=1)

    print(f"wrote {len(missions)} missions, "
          f"{len(stations['base'])}+{sum(len(v) for v in stations['dlc'].values())} stations, "
          f"{len(achievements)} achievements to {OUT}")


if __name__ == "__main__":
    main()
