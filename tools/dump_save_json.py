#!/usr/bin/env python3
"""Dump golden fixture data from the Python save implementation for Go tests.

Requires the apocalyptech/borderlands2 clone at ./borderlands2-tool.
Output: desktop/internal/bl2save/testdata/Save0001.golden.json
"""
import base64
import binascii
import hashlib
import json
import os
import struct
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "borderlands2-tool"))
sys.path.insert(0, ROOT)

from borderlands.datautil.protobuf import read_protobuf, write_protobuf  # noqa: E402
from borderlands.datautil.bitstreams import WriteBitstream  # noqa: E402
from borderlands.datautil.huffman import (  # noqa: E402
    make_huffman_tree, write_huffman_tree, huffman_compress, invert_tree,
)
from borderlands.datautil.lzo1x import lzo1x_1_compress  # noqa: E402
import save_io  # noqa: E402


def tree_to_jsonable(tree):
    out = {}
    for k, entries in tree.items():
        arr = []
        for wt, v in entries:
            if isinstance(v, (bytes, bytearray)):
                arr.append([wt, {"__b64__": base64.b64encode(bytes(v)).decode()}])
            elif isinstance(v, dict):
                arr.append([wt, tree_to_jsonable(v)])
            else:
                arr.append([wt, v])
        out[str(k)] = arr
    return out


def wrap_like_save_io(player_bytes):
    """Replicate save_io._write_save_locked's container bytes exactly."""
    crc = binascii.crc32(player_bytes) & 0xFFFFFFFF
    bitstream = WriteBitstream()
    tree = make_huffman_tree(player_bytes)
    write_huffman_tree(tree, bitstream)
    huffman_compress(invert_tree(tree), player_bytes, bitstream)
    data = bitstream.getvalue() + b"\x00\x00\x00\x00"
    header = struct.pack(">I3s", len(data) + 15, b"WSG")
    header += struct.pack("<III", 2, crc, len(player_bytes))
    compressed = lzo1x_1_compress(header + data)[1:]
    return hashlib.sha1(compressed).digest() + compressed


def synthetic_items():
    """Deterministic weapon/non-weapon items for golden item-layer tests."""
    def parts(count, mask, none_at):
        out = []
        for i in range(count):
            if i in none_at:
                out.append(None)
            else:
                out.append((i * 7 + 3) % mask)
        return out

    specs = [
        {"is_weapon": 1, "values": [1, 100, 50, 7, 72, 72] + parts(11, 2000, {9, 10}), "keys": [0, 123456789, -987654321]},
        {"is_weapon": 0, "values": [2, 300, 60, 5, 60, 60] + parts(11, 1000, {10}), "keys": [0, 42, -1]},
    ]
    items = []
    for spec in specs:
        for key in spec["keys"]:
            raw = save_io.wrap_item(spec["is_weapon"], spec["values"], key)
            code = "BL2(" + base64.b64encode(raw).decode("latin1") + ")"
            info = save_io.unwrap_item_info(raw)
            items.append({
                "is_weapon": spec["is_weapon"],
                "values": spec["values"],
                "key": key,
                "raw_b64": base64.b64encode(raw).decode("latin1"),
                "code": code,
                "info": info,
            })
    return items


def main():
    save_io.SAVE_DIR = os.path.join(ROOT, "tests")
    raw = open(os.path.join(save_io.SAVE_DIR, "Save0001.sav"), "rb").read()
    player_bytes = save_io.BaseApp.unwrap_player_data(raw)
    player = read_protobuf(player_bytes)

    reencoded = write_protobuf(player)
    pb_sha = hashlib.sha256(reencoded).hexdigest()
    container_sha = hashlib.sha256(wrap_like_save_io(player_bytes)).hexdigest()
    codes = save_io.export_all_codes("Save0001.sav")

    golden = {
        "protobuf_sha256": pb_sha,
        "container_sha256": container_sha,
        "tree": tree_to_jsonable(player),
        "gibbed_codes": codes,
        "synthetic_items": synthetic_items(),
    }
    out_dir = os.path.join(ROOT, "desktop", "internal", "bl2save", "testdata")
    os.makedirs(out_dir, exist_ok=True)
    out_path = os.path.join(out_dir, "Save0001.golden.json")
    with open(out_path, "w") as f:
        json.dump(golden, f, indent=1, sort_keys=True)
    items = sum(len(v) for v in codes.values())
    print(f"wrote {out_path} ({items} item codes)")


if __name__ == "__main__":
    main()
