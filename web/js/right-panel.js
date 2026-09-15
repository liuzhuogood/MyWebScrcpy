(() => {
  const main = document.querySelector('.player-main'); if (!main) return;
  let panel, splitter, active;
  function ensure() { if (panel) return; splitter=document.createElement('div'); splitter.className='right-panel-resizer'; panel=document.createElement('aside'); panel.className='right-panel'; panel.hidden=true; main.append(splitter,panel); splitter.onpointerdown=e=>{const start=e.clientX,w=panel.getBoundingClientRect().width; splitter.setPointerCapture(e.pointerId); const move=x=>panel.style.setProperty('--right-width',Math.max(420,Math.min(main.clientWidth-360,w+start-x.clientX))+'px'); const stop=()=>{splitter.onpointermove=null;splitter.onpointerup=null}; splitter.onpointermove=move;splitter.onpointerup=stop}; }
  window.RightPanel={open(node){ensure(); if(active&&active!==node) active.hidden=true; active=node; panel.append(node); node.hidden=false; panel.hidden=false; main.classList.add('right-panel-open'); window.dispatchEvent(new Event('resize'));},close(node){if(node)node.hidden=true;if(!node||node===active){active=null;panel.hidden=true;main.classList.remove('right-panel-open');window.dispatchEvent(new Event('resize'));}}};
})();
