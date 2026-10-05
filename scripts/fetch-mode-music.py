"""Developer-only asset import. Players use the embedded MP3s, fully offline.

Install imageio-ffmpeg into runtime/audio-tools, or pass --ffmpeg PATH.
Source recordings and their checksums are cached in ignored runtime/music-source.
"""
import argparse
import concurrent.futures
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
AUDIO = ROOT / 'internal/terminal/audio'
CACHE = ROOT / 'runtime/music-source'


def import_track(track, ffmpeg):
    original = CACHE / (track['mode'] + '.ogg')
    if not original.exists():
        for attempt in range(3):
            try:
                request = urllib.request.Request(track['download'], headers={'User-Agent': 'Table-Card-asset-import/1.0'})
                with urllib.request.urlopen(request, timeout=40) as response:
                    data = response.read()
                if not data.startswith(b'OggS'):
                    raise ValueError('Source is not an Ogg recording')
                original.write_bytes(data)
                break
            except Exception:
                if attempt == 2:
                    raise
                time.sleep(1 + attempt)
    temporary = CACHE / track['file']
    # Keep the whole composition at its original speed. Normalize loudness and
    # encode compact stereo MP3; the application still decodes it sequentially.
    subprocess.run([ffmpeg, '-hide_banner', '-loglevel', 'error', '-y', '-i', str(original),
                    '-map_metadata', '-1', '-vn', '-af', 'loudnorm=I=-20:TP=-2:LRA=7',
                    '-ar', '44100', '-ac', '2', '-c:a', 'libmp3lame', '-b:a', '96k',
                    '-metadata', 'title=' + track['title'], '-metadata', 'artist=' + track['author'],
                    str(temporary)], check=True)
    digest = hashlib.sha256(original.read_bytes()).hexdigest()
    encoded = temporary.read_bytes()
    (AUDIO / track['file']).write_bytes(encoded)
    print(f"Imported {track['mode']}: {len(encoded)} bytes", flush=True)
    return {'mode': track['mode'], 'source_sha256': digest,
            'mp3_sha256': hashlib.sha256(encoded).hexdigest(), 'bytes': len(encoded)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--ffmpeg')
    args = parser.parse_args()
    ffmpeg = args.ffmpeg
    if not ffmpeg:
        sys.path.insert(0, str(ROOT / 'runtime/audio-tools'))
        import imageio_ffmpeg
        ffmpeg = imageio_ffmpeg.get_ffmpeg_exe()
    tracks = json.loads((AUDIO / 'catalog.json').read_text(encoding='utf-8'))
    CACHE.mkdir(parents=True, exist_ok=True)
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        futures = [pool.submit(import_track, track, ffmpeg) for track in tracks]
        results = [future.result() for future in futures]
    (AUDIO / 'asset-checksums.json').write_text(json.dumps(results, indent=2) + '\n', encoding='utf-8')
    print(f"Imported all {len(results)} distinct mode tracks.", flush=True)


if __name__ == '__main__':
    main()
