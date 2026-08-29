#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/sample-videos"
mkdir -p "$DIR"

echo "Generating sample MP4s in $DIR"

ffmpeg -y -f lavfi -i "color=c=blue:s=640x360:d=8" -f lavfi -i "sine=f=440:d=8" \
  -shortest -c:v libx264 -pix_fmt yuv420p -c:a aac "$DIR/blue-8s.mp4" 2>/dev/null

ffmpeg -y -f lavfi -i "color=c=green:s=640x360:d=6" -f lavfi -i "sine=f=330:d=6" \
  -shortest -c:v libx264 -pix_fmt yuv420p -c:a aac "$DIR/green-6s.mp4" 2>/dev/null

ffmpeg -y -f lavfi -i "color=c=red:s=640x360:d=10" -f lavfi -i "sine=f=550:d=10" \
  -shortest -c:v libx264 -pix_fmt yuv420p -c:a aac "$DIR/red-10s.mp4" 2>/dev/null

echo "Done. Sample files:"
ls -lh "$DIR"/*.mp4
