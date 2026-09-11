(() => {
  const viewportInset = 12;
  const gap = 8;

  function place(panel, anchor) {
    if (panel.hidden || !anchor) return;

    const anchorRect = anchor.getBoundingClientRect();
    const panelRect = panel.getBoundingClientRect();
    const viewportWidth = document.documentElement.clientWidth;
    const viewportHeight = document.documentElement.clientHeight;
    const below = anchorRect.bottom + gap;
    const above = anchorRect.top - panelRect.height - gap;
    const top = below + panelRect.height <= viewportHeight - viewportInset || above < viewportInset ? below : above;
    const left = anchorRect.left + (anchorRect.width - panelRect.width) / 2;

    panel.style.left = `${Math.round(Math.min(Math.max(left, viewportInset), viewportWidth - panelRect.width - viewportInset))}px`;
    panel.style.top = `${Math.round(Math.min(Math.max(top, viewportInset), viewportHeight - panelRect.height - viewportInset))}px`;
    panel.dataset.popoverAnchor = anchor.id;
  }

  function placeOpenPopovers() {
    document.querySelectorAll('[data-popover-anchor]:not([hidden])').forEach(panel => {
      place(panel, document.getElementById(panel.dataset.popoverAnchor));
    });
  }

  window.placeToolbarPopover = place;
  window.addEventListener('resize', placeOpenPopovers);
  document.addEventListener('scroll', placeOpenPopovers, true);
})();
