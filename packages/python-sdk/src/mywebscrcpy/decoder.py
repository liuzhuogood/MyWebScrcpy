"""H.264 decoding with explicit packet metadata, no fabricated frames."""
from fractions import Fraction

from .dependencies import vision_dependencies
from .errors import ProtocolError


def annexb(data, length_size=4):
    if data.startswith((b'\x00\x00\x01', b'\x00\x00\x00\x01')):
        return data
    output, offset = bytearray(), 0
    while offset < len(data):
        if offset + length_size > len(data):
            raise ProtocolError('H.264 NAL 长度不完整')
        size = int.from_bytes(data[offset:offset + length_size], 'big')
        offset += length_size
        if size <= 0 or offset + size > len(data):
            raise ProtocolError('H.264 NAL 超出包边界')
        output.extend(b'\x00\x00\x00\x01' + data[offset:offset + size])
        offset += size
    if not output:
        raise ProtocolError('H.264 包为空')
    return bytes(output)


def configuration(data):
    if data.startswith((b'\x00\x00\x01', b'\x00\x00\x00\x01')):
        return data, 4
    if len(data) < 7 or data[0] != 1:
        raise ProtocolError('不支持的 H.264 配置')
    length_size, offset, output = (data[4] & 3) + 1, 6, bytearray()
    counts = [data[5] & 31]
    for group in range(2):
        if group:
            if offset >= len(data):
                raise ProtocolError('缺少 PPS')
            counts.append(data[offset]); offset += 1
        for _ in range(counts[group]):
            if offset + 2 > len(data):
                raise ProtocolError('配置 NAL 长度缺失')
            size = int.from_bytes(data[offset:offset + 2], 'big'); offset += 2
            if size <= 0 or offset + size > len(data):
                raise ProtocolError('配置 NAL 超出边界')
            output.extend(b'\x00\x00\x00\x01' + data[offset:offset + size]); offset += size
    if not all(counts):
        raise ProtocolError('配置必须包含 SPS/PPS')
    return bytes(output), length_size


class Decoder:
    def __init__(self):
        self.av, _, _ = vision_dependencies()
        self.codec = None
        self.waiting_key = True
        self.length_size = 4
        self.metadata = {}

    def reset(self):
        self.codec = None
        self.metadata.clear()
        self.waiting_key = True

    def configure(self, payload):
        self.reset()
        data, self.length_size = configuration(payload)
        self.codec = self.av.CodecContext.create('h264', 'r')
        self.codec.thread_count = 1
        self.codec.extradata = data

    def decode(self, kind, payload, metadata):
        if self.codec is None:
            raise ProtocolError('视频包先于配置到达')
        if self.waiting_key and kind != 1:
            return []  # Prediction before the first key frame cannot be decoded.
        self.waiting_key = False
        packet = self.av.Packet(annexb(payload, self.length_size))
        packet.pts = metadata['frame_id']
        packet.time_base = Fraction(1, 1000000)
        self.metadata[packet.pts] = metadata
        if len(self.metadata) > 128:
            raise ProtocolError('解码帧延迟超过元数据上限')
        try:
            decoded = self.codec.decode(packet)
        except self.av.error.FFmpegError as error:
            raise ProtocolError('H.264 解码失败') from error
        result = []
        for frame in decoded:
            if frame.pts not in self.metadata:
                raise ProtocolError('解码帧无法关联来源包')
            meta = self.metadata.pop(frame.pts)
            result.append((meta, frame.to_ndarray(format='bgr24')))
        return result
