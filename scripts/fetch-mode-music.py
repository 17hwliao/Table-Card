"""Developer-only asset import. Players use the embedded MP3s, fully offline.

Install imageio-ffmpeg into runtime/audio-tools, or pass --ffmpeg PATH.
Source recordings and their checksums are cached in ignored runtime/music-source.
"""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
AUDIO = ROOT / 'internal/terminal/audio'
CACHE = ROOT / 'runtime/music-source'


def atomic_write(path, data):
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as temporary:
        temporary.write(data)
        temporary_path = Path(temporary.name)
    try:
        os.replace(temporary_path, path)
    finally:
        temporary_path.unlink(missing_ok=True)


def source_recording(track, refresh=False):
    url = track['download']
    source_key = hashlib.sha256(url.encode('utf-8')).hexdigest()
    original = CACHE / (track['mode'] + '-' + source_key + '.ogg')
    manifest = original.with_suffix('.json')
    valid = False
    if not refresh and original.is_file() and manifest.is_file():
        try:
            data = original.read_bytes()
            metadata = json.loads(manifest.read_text(encoding='utf-8'))
            valid = (data.startswith(b'OggS') and metadata['download'] == url
                     and metadata['sha256'] == hashlib.sha256(data).hexdigest())
        except (OSError, ValueError, KeyError, TypeError):
            valid = False
    if not valid:
        for attempt in range(3):
            try:
                request = urllib.request.Request(url, headers={'User-Agent': 'Table-Card-asset-import/1.0'})
                with urllib.request.urlopen(request, timeout=40) as response:
                    data = response.read()
                if not data.startswith(b'OggS'):
                    raise ValueError('Source is not an Ogg recording')
                atomic_write(original, data)
                metadata = {'download': url, 'sha256': hashlib.sha256(data).hexdigest()}
                atomic_write(manifest, (json.dumps(metadata, indent=2) + '\n').encode('utf-8'))
                break
            except Exception:
                if attempt == 2:
                    raise
                time.sleep(1 + attempt)
    return original


def import_track(track, ffmpeg, stage, refresh=False):
    original = source_recording(track, refresh)
    temporary = stage / track['file']
    # Keep the whole composition at its original speed. Normalize loudness and
    # encode compact stereo MP3; the application still decodes it sequentially.
    subprocess.run([ffmpeg, '-hide_banner', '-loglevel', 'error', '-y', '-i', str(original),
                    '-map_metadata', '-1', '-vn', '-af', 'loudnorm=I=-20:TP=-2:LRA=7',
                    '-ar', '44100', '-ac', '2', '-c:a', 'libmp3lame', '-b:a', '96k',
                    '-metadata', 'title=' + track['title'], '-metadata', 'artist=' + track['author'],
                    str(temporary)], check=True)
    digest = hashlib.sha256(original.read_bytes()).hexdigest()
    encoded = temporary.read_bytes()
    print(f"Prepared {track['mode']}: {len(encoded)} bytes", flush=True)
    return {'mode': track['mode'], 'source_sha256': digest,
            'mp3_sha256': hashlib.sha256(encoded).hexdigest(), 'bytes': len(encoded)}


def install_tracks(stage, tracks, results):
    # Publish only after every conversion succeeds. Back up the previous set so
    # an interrupted filesystem write can be rolled back along with its manifest.
    checksum_name = 'asset-checksums.json'
    (stage / checksum_name).write_text(json.dumps(results, indent=2) + '\n', encoding='utf-8')
    names = [track['file'] for track in tracks] + [checksum_name]
    backup = stage / 'backup'
    backup.mkdir()
    existing = set()
    for name in names:
        if (AUDIO / name).is_file():
            shutil.copyfile(AUDIO / name, backup / name)
            existing.add(name)
    installed = []
    try:
        for name in names:
            os.replace(stage / name, AUDIO / name)
            installed.append(name)
    except OSError:
        for name in reversed(installed):
            if name in existing:
                os.replace(backup / name, AUDIO / name)
            else:
                (AUDIO / name).unlink(missing_ok=True)
        raise


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--ffmpeg')
    parser.add_argument('--refresh', action='store_true', help='Redownload sources even when their cache is valid')
    args = parser.parse_args()
    ffmpeg = args.ffmpeg
    if not ffmpeg:
        sys.path.insert(0, str(ROOT / 'runtime/audio-tools'))
        import imageio_ffmpeg
        ffmpeg = imageio_ffmpeg.get_ffmpeg_exe()
    tracks = json.loads((AUDIO / 'catalog.json').read_text(encoding='utf-8'))
    for field in ('mode', 'file'):
        values = [track[field] for track in tracks]
        if len(set(values)) != len(values) or any(not value or Path(value).name != value or '/' in value or '\\' in value for value in values):
            raise ValueError(f'Catalog {field} values must be unique, plain filenames')
    if any(not track['file'].endswith('.mp3') for track in tracks):
        raise ValueError('Catalog audio filenames must end with .mp3')
    CACHE.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='import-', dir=CACHE) as directory:
        stage = Path(directory)
        with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
            futures = [pool.submit(import_track, track, ffmpeg, stage, args.refresh) for track in tracks]
            results = [future.result() for future in futures]
        install_tracks(stage, tracks, results)
    print(f"Imported all {len(results)} distinct mode tracks.", flush=True)


if __name__ == '__main__':
    main()
