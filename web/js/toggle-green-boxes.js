(function() {
  const toggleBtn = document.getElementById('btn-toggle-green-boxes');
  const detectionOverlay = document.getElementById('detection-overlay');
  const uiInspectorOverlay = document.getElementById('ui-inspector-overlay');

  if (toggleBtn) {
    const savedState = localStorage.getItem('show-green-boxes');
    if (savedState !== null) {
      toggleBtn.checked = savedState === 'true';
    }

    function updateVisibility() {
      const show = toggleBtn.checked;
      if (detectionOverlay) {
        detectionOverlay.style.visibility = show ? 'visible' : 'hidden';
      }
      if (uiInspectorOverlay) {
        uiInspectorOverlay.style.visibility = show ? 'visible' : 'hidden';
      }
      localStorage.setItem('show-green-boxes', show);
    }

    // Initialize state
    updateVisibility();

    toggleBtn.addEventListener('change', updateVisibility);
  }
})();
