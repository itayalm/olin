#!/bin/sh

set -e
set -x

zig build-lib -target wasm32-freestanding-none src/coi.zig
zig build-lib -target wasm32-freestanding-none --release-fast src/cat.zig
zig build-lib -target wasm32-freestanding-none --release-fast src/httptest.zig
zig build-lib -target wasm32-freestanding-none --release-fast src/shaman.zig
zig build-lib -target wasm32-freestanding-none --release-fast src/cwagi.zig
zig build-lib -target wasm32-freestanding-none --release-fast src/triangle.zig
cwa -vm-stats -test coi.wasm a b c d
zig build-lib -target wasm32-freestanding-none --release-fast src/coi.zig
cwa -vm-stats -test coi.wasm a b c d
cwa -vm-stats triangle.wasm

rm *.h *.o
