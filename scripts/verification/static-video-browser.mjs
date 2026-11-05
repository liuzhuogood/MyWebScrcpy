// Run with ego-browser nodejs. Requires the opt-in Go WebSocket fixture and Vite5192.
const task=await taskSpace('MyWebScrcpy 静止画面回退排查')
const page=task.page('p1')
await page.goto('http://127.0.0.1:5192/')
await page.evaluate(async()=>{
 const {createApp,h,nextTick}=await import('/node_modules/.vite/deps/vue.js')
 const {usePlayer}=await import('/src/scrcpy/usePlayer.ts')
 document.querySelector('#app').__vue_app__.unmount()
 window.__staticFrames=[]
 const NativeWebSocket=window.WebSocket
 window.__ignoreHeartbeats=false
 window.WebSocket=class extends NativeWebSocket {
  constructor(...args) {
   super(...args)
   this.addEventListener('message',event=>{
    if (window.__ignoreHeartbeats && typeof event.data==='string' && JSON.parse(event.data).type==='stream.heartbeat') event.stopImmediatePropagation()
   })
  }
 }
 let player
 const app=createApp({setup(){
  player=usePlayer('static-fixture',{onFrame:()=>{
   const canvas=player.canvasRef.value
   window.__staticFrames.push({at:performance.now(),pixel:[...canvas.getContext('2d').getImageData(80,48,1,1).data]})
  }})
  return()=>h('div',{ref:player.stageRef,style:'height:500px;width:600px'},h('main',{'data-player-viewport':'',style:'height:100%;width:100%'},h('canvas',{ref:player.canvasRef,'aria-label':'静止视频测试画布'})))
 }})
 app.mount('#app');window.__staticPlayer=player;window.__staticApp=app;await nextTick()
})
await page.waitForFunction(async()=>{const s=await(await fetch('/api/static-video/status')).json();return s.connections>=1})
await page.fetch('/api/static-video/start',{method:'POST'})
await page.waitForFunction(()=>window.__staticFrames.some(frame=>frame.pixel[2]>200&&frame.pixel[0]<30))
console.log({stage:'blue-rendered',frames:await page.evaluate(()=>window.__staticFrames)})
