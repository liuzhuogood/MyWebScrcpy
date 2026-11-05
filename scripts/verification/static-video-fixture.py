"""Generate the one-keyframe red→blue H.264 source for the Go/browser fixture.

python3 scripts/verification/static-video-fixture.py
MYWEBSCRCPY_STATIC_FRAMES=<absolute output>/frames.json go test ./internal/ws \
    -run '^TestStaticVideoBrowserFixture$' -v -timeout 12m

Use Vite on 5192 with backend proxy 21992, then run the two static-video
browser/assert .mjs files with ego-browser nodejs. No ADB/device is involved.
"""
import base64
import json
from pathlib import Path
import re
import subprocess

output = Path(__file__).resolve().parents[2] / '.cache/verification/static-video'
output.mkdir(parents=True, exist_ok=True)
video = output / 'red-blue.h264'
subprocess.run([
    'ffmpeg', '-hide_banner', '-loglevel', 'error',
    '-f', 'lavfi', '-i', 'color=red:s=160x96:r=5:d=1',
    '-f', 'lavfi', '-i', 'color=blue:s=160x96:r=5:d=1',
    '-filter_complex', '[0:v][1:v]concat=n=2:v=1:a=0',
    '-c:v', 'libx264', '-profile:v', 'baseline', '-pix_fmt', 'yuv420p',
    '-x264-params', 'keyint=100:min-keyint=100:scenecut=0:bframes=0:aud=1',
    '-f', 'h264', '-y', str(video),
], check=True)
packets = json.loads(subprocess.check_output([
    'ffprobe', '-v', 'error', '-show_packets', '-show_entries',
    'packet=pos,size,flags', '-of', 'json', str(video),
]))['packets']
raw = video.read_bytes()
nals = re.split(b'\x00\x00(?:\x00)?\x01', raw[:int(packets[0]['size'])])
config = b''.join(b'\0\0\0\1' + nal for nal in nals if nal and nal[0] & 31 in (7, 8))
assert config, 'encoder produced no SPS/PPS'
frames = [{'Kind': 0, 'PTS': 0, 'Payload': base64.b64encode(config).decode()}]
for index, packet in enumerate(packets):
    payload = raw[int(packet['pos']):int(packet['pos']) + int(packet['size'])]
    frames.append({'Kind': 1 if 'K' in packet['flags'] else 2,
                   'PTS': index * 200000, 'Payload': base64.b64encode(payload).decode()})
assert len(frames) == 11 and sum(frame['Kind'] == 1 for frame in frames) == 1
(output / 'frames.json').write_text(json.dumps(frames) + '\n')
print(output / 'frames.json')
