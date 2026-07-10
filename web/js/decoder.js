// decoder.js — WebCodecs VideoDecoder 封装，用于解码 scrcpy 的 H.264 裸流
//
// scrcpy 的视频帧格式（Go 端 pumpVideo 推来的二进制帧）：
//   [1B kind][8B pts BE][payload...]
//     kind 0=config(SPS/PPS)  1=key(IDR)  2=delta(P)  3=session(尺寸变化)
//   session 帧的 payload 是 8 字节: [4B width][4B height]

class ScrcpyDecoder {
  constructor(canvas) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.decoder = null;
    this.codec = 'h264';
    this.configured = false;
    this.frameCount = 0;
    this.lastFrameTime = 0;
  }

  // 根据 codec 名字构造 WebCodecs codec 字符串
  codecString() {
    // H.264 Baseline，具体 profile 由 config 包里的 SPS 决定，这里给一个通用值
    if (this.codec === 'h264') return 'avc1.42E01F';
    if (this.codec === 'h265') return 'hvc1.1.6.L93.B0';
    if (this.codec === 'av1') return 'av01.0.04M.08';
    return 'avc1.42E01F';
  }

  configure(codec, width, height) {
    this.codec = codec;
    this.canvas.width = width;
    this.canvas.height = height;

    if (typeof VideoDecoder === 'undefined') {
      throw new Error('此浏览器不支持 WebCodecs，请用 Chrome 94+');
    }

    if (this.decoder) {
      this.decoder.close();
    }

    this.decoder = new VideoDecoder({
      output: (frame) => this.onFrame(frame),
      error: (e) => console.error('[decoder] error', e),
    });
    this.configured = false;
    console.log(`[decoder] canvas ${width}x${height}, codec ${codec}, 等待 config 包...`);
  }

  // 处理一帧二进制数据
  // data: Uint8Array, 格式见文件头注释: [1B kind][8B pts BE][payload...]
  handleFrame(data) {
    const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
    const kind = view.getUint8(0);
    const ptsNum = Number(view.getBigUint64(1, false));
    const payload = data.subarray(9);

    try {
      switch (kind) {
        case 0: this.onConfig(payload); break;
        case 1: this.decodeChunk(payload, true, ptsNum); break;
        case 2: this.decodeChunk(payload, false, ptsNum); break;
        case 3: this.onResize(payload); break;
      }
    } catch(e) {
      console.error('[decoder] handleFrame error:', e);
    }
  }

  onConfig(payload) {
    // scrcpy 的 config 包是 SPS/PPS。
    // WebCodecs 的 configure 需要 description（AVCC 格式的 codec 配置）。
    // scrcpy MediaCodec 输出的 config 通常是 AVCC 格式（带 4 字节长度前缀），
    // 但也可能是 Annex-B（带 00 00 00 01 起始码）。这里做自动检测和转换。
    const description = this.toAVCC(payload);

    // 每次收到 config 包都重新 configure。
    // 设备旋转时 scrcpy 会重建编码器并发送新的 config 包，
    // 如果不重新 configure，旧 decoder 无法解码新编码器的输出。
    if (this.decoder.state === 'configured') {
      // 已配置过，先 reset 再重新 configure
      this.decoder.reset();
    }
    this.decoder.configure({
      codec: this.codecString(),
      description: description,
      optimizeForLatency: true,
    });
    this.configured = true;
    console.log('[decoder] decoder 已 configure, description size=', description.length);
  }

  // 将可能的 Annex-B 格式 SPS/PPS 转为 AVCC (avcC) description
  // 如果已经是 AVCC（长度前缀），直接返回。
  toAVCC(payload) {
    // 检测起始码 00 00 00 01 或 00 00 01
    if (payload.length >= 4 &&
        payload[0] === 0 && payload[1] === 0 &&
        payload[2] === 0 && payload[3] === 1) {
      // Annex-B 格式，需要解析出 SPS/PPS 并构造 avcC
      return this.annexBToAvcC(payload);
    }
    // 否则假设已经是 AVCC length-prefixed 格式，直接用
    return payload;
  }

  // 解析 Annex-B 流，提取 NAL，构造 avcC box 内容
  annexBToAvcC(data) {
    const nals = this.splitNAL(data);
    let sps = null, pps = null;
    for (const nal of nals) {
      if (nal.length === 0) continue;
      const nalType = nal[0] & 0x1F;
      if (nalType === 7) sps = nal;       // SPS
      else if (nalType === 8) pps = nal;   // PPS
    }
    if (!sps || !pps) {
      console.warn('[decoder] Annex-B 中未找到 SPS/PPS, nals=', nals.length);
      return data;
    }
    // avcC 格式:
    //   01 <profile> <compat> <level> FF E1 <spsLen 2B> <sps> 01 <ppsLen 2B> <pps>
    const buf = new Uint8Array(11 + sps.length + pps.length);
    let i = 0;
    buf[i++] = 1;
    buf[i++] = sps[1]; // profile_idc
    buf[i++] = sps[2]; // constraint flags
    buf[i++] = sps[3]; // level_idc
    buf[i++] = 0xFF;   // 111111 + reserved(2 bits) + NALU length size - 1 (4 bytes)
    buf[i++] = 0xE1;   // 111 + reserved(3 bits) + num SPS (1)
    buf[i++] = (sps.length >> 8) & 0xFF;
    buf[i++] = sps.length & 0xFF;
    buf.set(sps, i); i += sps.length;
    buf[i++] = 1;      // num PPS
    buf[i++] = (pps.length >> 8) & 0xFF;
    buf[i++] = pps.length & 0xFF;
    buf.set(pps, i);
    console.log('[decoder] Annex-B → avcC 转换完成, profile=', sps[1], 'level=', sps[3]);
    return buf;
  }

  // 按 00 00 00 01 / 00 00 01 起始码切分 NAL，返回去掉起始码的 NAL 数组
  splitNAL(data) {
    const nals = [];
    let i = 0;
    while (i < data.length) {
      // 找下一个起始码
      let start = this.findStartCode(data, i);
      if (start === -1) break;
      // 找再下一个起始码（当前 NAL 的结束）
      let end = this.findStartCode(data, start + 3);
      if (end === -1) end = data.length;
      // 跳过起始码长度
      let nalStart = start;
      if (data[nalStart+2] === 1) nalStart += 3;
      else if (data[nalStart+3] === 1) nalStart += 4;
      nals.push(data.subarray(nalStart, end));
      i = end;
    }
    return nals;
  }

  findStartCode(data, from) {
    for (let i = from; i < data.length - 3; i++) {
      if (data[i] === 0 && data[i+1] === 0) {
        if (data[i+2] === 1) return i;          // 00 00 01
        if (data[i+2] === 0 && data[i+3] === 1) return i; // 00 00 00 01
      }
    }
    return -1;
  }

  decodeChunk(payload, isKey, pts) {
    if (!this.configured || this.decoder.state !== 'configured') return;

    // scrcpy 的 media 包 payload 可能是 AVCC（长度前缀）或 Annex-B（起始码）。
    // WebCodecs 的 EncodedVideoChunk 可以直接接受 AVCC length-prefixed 格式。
    // 如果是 Annex-B，需要转成 length-prefixed。
    let chunkData = payload;
    if (payload.length >= 4 &&
        payload[0] === 0 && payload[1] === 0 &&
        payload[2] === 0 && payload[3] === 1) {
      chunkData = this.annexBToLengthPrefixed(payload);
    }

    const chunk = new EncodedVideoChunk({
      type: isKey ? 'key' : 'delta',
      timestamp: pts,        // 微秒
      data: chunkData,
    });
    // decode() 返回 Promise，必须 catch 否则 unhandled rejection 可能停止 WS dispatch
    try {
      this.decoder.decode(chunk).catch(e => {
        // 解码错误通常是帧损坏或缺少关键帧，静默忽略
      });
    } catch(e) {
      // 同步异常（如 decoder 状态错误），忽略
    }
  }

  // 将 Annex-B (起始码分隔) 转为 AVCC (4字节大端长度前缀)
  annexBToLengthPrefixed(data) {
    const nals = this.splitNAL(data);
    let totalLen = 0;
    for (const nal of nals) totalLen += 4 + nal.length;
    const buf = new Uint8Array(totalLen);
    let off = 0;
    for (const nal of nals) {
      const view = new DataView(buf.buffer, off, 4);
      view.setUint32(0, nal.length, false);
      buf.set(nal, off + 4);
      off += 4 + nal.length;
    }
    return buf;
  }

  onResize(payload) {
    const view = new DataView(payload.buffer, payload.byteOffset, payload.byteLength);
    const w = view.getUint32(0, false);
    const h = view.getUint32(4, false);
    console.log(`[decoder] 尺寸变化: ${w}x${h}`);
    this.canvas.width = w;
    this.canvas.height = h;
    // 通知外部更新 packer 的设备尺寸（坐标映射用）
    if (this.onDeviceResize) {
      this.onDeviceResize(w, h);
    }
  }

  onFrame(frame) {
    this.ctx.drawImage(frame, 0, 0, this.canvas.width, this.canvas.height);
    frame.close();
    this.frameCount++;
    // 通知外部（用于息屏检测）
    if (this.onFrameDecoded) this.onFrameDecoded();
    const now = performance.now();
    if (now - this.lastFrameTime > 1000) {
      const fps = (this.frameCount * 1000 / (now - this.lastFrameTime)).toFixed(1);
      this.onFps && this.onFps(fps);
      this.frameCount = 0;
      this.lastFrameTime = now;
    }
  }

  close() {
    if (this.decoder) {
      this.decoder.close();
      this.decoder = null;
    }
  }
}
