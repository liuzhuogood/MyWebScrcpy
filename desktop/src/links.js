// Dispatch blank-target links directly; native popup callbacks vary by WebView.
document.addEventListener('click', (event) => {
  const link = event.target instanceof Element ? event.target.closest('a[href]') : null;
  if (!link || link.hasAttribute('download') || event.defaultPrevented) return;
  const destination = new URL(link.href, location.href);
  if (link.target !== '_blank' && destination.origin === location.origin) return;
  if (destination.protocol !== 'https:' && destination.origin !== location.origin) return;
  event.preventDefault();
  window.__TAURI_INTERNALS__.invoke('open_browser_link', { url: destination.href })
    .catch((error) => console.error('[Desktop] 打开浏览器链接失败:', error));
}, true);
