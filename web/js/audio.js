(() => {
  const tools = document.querySelector('.player-toolbar .tools');
  const moreButton = document.getElementById('btn-more');
  if (!tools || !moreButton) return;
  const supportsAudioDecoder = 'AudioDecoder' in window;

  const button = document.createElement('button');
  button.id = 'btn-audio'; button.type = 'button'; button.title = '设备声音不可用';
  button.setAttribute('aria-label', '设备声音');
  button.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H2v6h4l5 4z"/><path d="M15.5 8.5a5 5 0 0 1 0 7"/><path d="M19 5a10 10 0 0 1 0 14"/></svg>';
  button.disabled = true;
  tools.insertBefore(button, moreButton);

  const panel = document.createElement('div');
  panel.className = 'audio-volume-panel'; panel.hidden = true;
  panel.setAttribute('role', 'group'); panel.setAttribute('aria-label', '设备声音音量');
  panel.innerHTML = '<input type="range" min="0" max="100" value="100" aria-label="设备声音音量"><output>100%</output>';
  document.querySelector('.player-toolbar').append(panel);
  const volume = panel.querySelector('input');
  const volumeOutput = panel.querySelector('output');

  let decoder, context, gain, config, muted = true, level = 1, anchorPTS, anchorTime, status = 'unavailable';
  const sampleRates = [96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350];
  const setStatus = (next, reason) => {
    status = next;
    const ready = next === 'ready';
    button.disabled = !ready;
    button.classList.toggle('is-muted', muted || !ready);
    button.title = ready ? (muted ? '点击播放设备声音' : '点击静音设备声音') : (reason || '设备声音不可用');
  };
  if (!supportsAudioDecoder) setStatus('decoder_error', '当前浏览器不支持设备音频播放');
  const parseASC = bytes => {
    if (bytes.length < 2) throw Error('AAC 配置不完整');
    let objectType = bytes[0] >> 3, frequencyIndex = ((bytes[0] & 7) << 1) | (bytes[1] >> 7);
    if (objectType === 31) { objectType = 32 + ((bytes[1] & 0x7f) << 1) + (bytes[2] >> 7); }
    const channels = (bytes[1] >> 3) & 15;
    const sampleRate = frequencyIndex === 15 ? ((bytes[1] & 7) << 21) | (bytes[2] << 13) | (bytes[3] << 5) | (bytes[4] >> 3) : sampleRates[frequencyIndex];
    if (!sampleRate || !channels || !objectType) throw Error('不支持的 AAC 配置');
    return { codec: `mp4a.40.${objectType}`, sampleRate, numberOfChannels: channels, description: bytes };
  };
  const resetDecoder = description => {
    if (!supportsAudioDecoder) throw Error('当前浏览器不支持设备音频播放');
    try { decoder?.close(); } catch (_) {}
    config = parseASC(description);
    decoder = new AudioDecoder({ output: outputAudio, error: error => setStatus('decoder_error', `音频解码失败：${error.message || error}`) });
    decoder.configure(config);
    anchorPTS = null; anchorTime = null;
    setStatus('ready');
  };
  const ensureContext = async () => {
    if (!context) {
      context = new AudioContext(); gain = context.createGain(); gain.connect(context.destination);
    }
    if (context.state !== 'running') await context.resume();
    gain.gain.value = muted ? 0 : level;
  };
  const outputAudio = audio => {
    try {
      if (!context || muted) return;
      const now = context.currentTime;
      const pts = Number(audio.timestamp) / 1e6;
      if (anchorPTS === null || Math.abs((pts - anchorPTS) - (now - anchorTime)) > .35) { anchorPTS = pts; anchorTime = now + .08; }
      const start = Math.max(now + .01, anchorTime + (pts - anchorPTS));
      const buffer = context.createBuffer(audio.numberOfChannels, audio.numberOfFrames, audio.sampleRate);
      for (let channel = 0; channel < audio.numberOfChannels; channel++) audio.copyTo(buffer.getChannelData(channel), { planeIndex: channel, format: 'f32-planar' });
      const source = context.createBufferSource(); source.buffer = buffer; source.connect(gain); source.start(start);
    } finally { audio.close(); }
  };
  button.addEventListener('click', async () => {
    if (status !== 'ready') return;
    muted = !muted;
    try { await ensureContext(); setStatus('ready'); panel.hidden = false; } catch (_) { setStatus('decoder_error', '浏览器未允许播放设备声音'); }
  });
  volume.addEventListener('input', () => {
    level = Number(volume.value) / 100;
    volumeOutput.textContent = `${volume.value}%`;
    muted = level === 0;
    if (gain) gain.gain.value = muted ? 0 : level;
    setStatus('ready');
  });
  volume.addEventListener('keydown', event => event.stopPropagation());
  document.addEventListener('click', event => {
    if (!panel.contains(event.target) && event.target !== button && !button.contains(event.target)) panel.hidden = true;
  });
  window.deviceAudio = {
    status(message) { if (message.status !== 'connecting') setStatus(message.status, message.reason); },
    handlePacket(data) {
      if (!supportsAudioDecoder) return;
      const bytes = new Uint8Array(data);
      if (bytes.length < 10) return;
      const configPacket = bytes[1] === 1;
      const pts = new DataView(bytes.buffer, bytes.byteOffset + 2, 8).getBigUint64(0);
      const payload = bytes.slice(10);
      try {
        if (configPacket) resetDecoder(payload);
        else if (decoder && status === 'ready') decoder.decode(new EncodedAudioChunk({ type: 'key', timestamp: Number(pts), data: payload }));
      } catch (error) { setStatus('decoder_error', `音频不可播放：${error.message || error}`); }
    },
  };
})();
