(() => {
  const button = document.getElementById('btn-picture-in-picture');
  const canvas = document.getElementById('screen');
  if (!button || !canvas) return;

  const videoPrototype = window.HTMLVideoElement?.prototype;
  const supportsStandardVideoPiP = typeof canvas.captureStream === 'function'
    && typeof videoPrototype?.requestPictureInPicture === 'function';
  const supportsWebkitVideoPiP = typeof canvas.captureStream === 'function'
    && typeof videoPrototype?.webkitSetPresentationMode === 'function';
  const documentPiP = window.documentPictureInPicture;
  const supportsDocumentPiP = typeof documentPiP?.requestWindow === 'function';
  const supported = supportsStandardVideoPiP || supportsWebkitVideoPiP || supportsDocumentPiP;
  let video = null;
  let stream = null;
  let pipWindow = null;
  let pictureCanvas = null;
  let drawFrame = 0;
  let webkitPiPActive = false;

  const setState = (active, message) => {
    button.disabled = false;
    button.classList.toggle('is-active', active);
    button.title = active ? '退出画中画' : (message || '画中画');
    button.setAttribute('aria-label', button.title);
  };

  const showFailure = message => {
    setState(false, message);
    window.alert(`画中画无法打开：${message}`);
  };

  const cleanup = () => {
    cancelAnimationFrame(drawFrame);
    drawFrame = 0;
    pictureCanvas = null;
    pipWindow = null;
    webkitPiPActive = false;
    if (video) {
      video.srcObject = null;
      video.remove();
      video = null;
    }
    stream?.getTracks().forEach(track => track.stop());
    stream = null;
    setState(false);
  };

  const waitForVideoFrame = source => new Promise((resolve, reject) => {
    const finish = () => {
      clearTimeout(timeout);
      source.removeEventListener('loadedmetadata', check);
      source.removeEventListener('canplay', check);
    };
    const check = () => {
      if (!source.videoWidth || !source.videoHeight) return;
      finish();
      resolve();
    };
    const timeout = setTimeout(() => {
      finish();
      reject(Error('画中画画面未准备完成'));
    }, 2500);
    source.addEventListener('loadedmetadata', check);
    source.addEventListener('canplay', check);
    check();
  });

  const drawDocumentPiP = () => {
    if (!pipWindow || pipWindow.closed || !pictureCanvas) {
      cleanup();
      return;
    }
    if (pictureCanvas.width !== canvas.width || pictureCanvas.height !== canvas.height) {
      pictureCanvas.width = canvas.width;
      pictureCanvas.height = canvas.height;
    }
    pictureCanvas.getContext('2d').drawImage(canvas, 0, 0);
    drawFrame = requestAnimationFrame(drawDocumentPiP);
  };

  const openDocumentPiP = async () => {
    if (!supportsDocumentPiP) throw Error('当前浏览器不支持文档画中画');
    const ratio = canvas.width / canvas.height;
    let width = Math.min(480, Math.max(240, canvas.width));
    let height = Math.round(width / ratio);
    if (height > 720) { height = 720; width = Math.round(height * ratio); }
    pipWindow = await documentPiP.requestWindow({ width, height });
    pipWindow.document.body.innerHTML = '<canvas aria-label="设备投屏画面"></canvas>';
    pipWindow.document.head.insertAdjacentHTML('beforeend', '<style>html,body{width:100%;height:100%;margin:0;background:#000;overflow:hidden}canvas{display:block;width:100%;height:100%;object-fit:contain}</style>');
    pictureCanvas = pipWindow.document.querySelector('canvas');
    pipWindow.addEventListener('pagehide', cleanup, { once: true });
    drawDocumentPiP();
    setState(true);
  };

  const openVideoPiP = async () => {
    stream = canvas.captureStream(30);
    video = document.createElement('video');
    video.muted = true;
    video.playsInline = true;
    video.className = 'picture-in-picture-source';
    video.srcObject = stream;
    document.body.append(video);
    video.addEventListener('webkitpresentationmodechanged', () => {
      if (webkitPiPActive && video?.webkitPresentationMode !== 'picture-in-picture') cleanup();
    });
    await video.play();
    await waitForVideoFrame(video);
    if (supportsStandardVideoPiP) await video.requestPictureInPicture();
    else {
      video.webkitSetPresentationMode('picture-in-picture');
      webkitPiPActive = true;
    }
    setState(true);
  };

  const open = async () => {
    if (!canvas.width || !canvas.height) {
      showFailure('投屏画面尚未准备好');
      return;
    }
    try {
      if (supportsStandardVideoPiP || supportsWebkitVideoPiP) await openVideoPiP();
      else await openDocumentPiP();
    } catch (videoError) {
      cleanup();
      try {
        await openDocumentPiP();
      } catch (documentError) {
        console.warn('[picture-in-picture] 打开失败:', videoError, documentError);
        cleanup();
        showFailure('当前浏览器或页面设置不支持该功能');
      }
    }
  };

  const close = async () => {
    const activeVideo = video;
    if (pipWindow && !pipWindow.closed) pipWindow.close();
    if (activeVideo && document.pictureInPictureElement === activeVideo) {
      try {
        await document.exitPictureInPicture();
      } catch (error) {
        if (error.name !== 'InvalidStateError') console.warn('[picture-in-picture] 关闭失败:', error);
      }
    } else if (activeVideo?.webkitPresentationMode === 'picture-in-picture') {
      activeVideo.webkitSetPresentationMode('inline');
    }
    cleanup();
  };

  button.addEventListener('click', async () => {
    if ((video && document.pictureInPictureElement === video)
      || video?.webkitPresentationMode === 'picture-in-picture' || pipWindow) {
      await close();
      return;
    }
    cleanup();
    await open();
  });

  document.addEventListener('leavepictureinpicture', event => {
    if (event.target === video) cleanup();
  });

  setState(false, supported ? undefined : '尝试打开画中画');
})();
