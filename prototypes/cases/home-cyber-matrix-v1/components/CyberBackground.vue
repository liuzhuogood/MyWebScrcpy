<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const canvasRef = ref<HTMLCanvasElement | null>(null)
let animationId: number | null = null

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  alpha: number
  color: string
}

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  let width = (canvas.width = window.innerWidth)
  let height = (canvas.height = window.innerHeight)

  const handleResize = () => {
    if (!canvas) return
    width = canvas.width = window.innerWidth
    height = canvas.height = window.innerHeight
  }
  window.addEventListener('resize', handleResize)

  // 粒子系统
  const particles: Particle[] = []
  const particleCount = 45
  const colors = ['#00f2fe', '#4facfe', '#00ff87', '#38ef7d']

  for (let i = 0; i < particleCount; i++) {
    particles.push({
      x: Math.random() * width,
      y: Math.random() * height,
      vx: (Math.random() - 0.5) * 0.6,
      vy: (Math.random() - 0.5) * 0.6,
      size: Math.random() * 2 + 1,
      alpha: Math.random() * 0.5 + 0.2,
      color: colors[Math.floor(Math.random() * colors.length)],
    })
  }

  let mouseX = -1000
  let mouseY = -1000
  const handleMouseMove = (e: MouseEvent) => {
    mouseX = e.clientX
    mouseY = e.clientY
  }
  window.addEventListener('mousemove', handleMouseMove)

  let gridOffset = 0

  const render = () => {
    ctx.clearRect(0, 0, width, height)

    // 绘制深空背景微弱光晕
    const radialGrad = ctx.createRadialGradient(
      width / 2,
      height / 3,
      10,
      width / 2,
      height / 2,
      Math.max(width, height) * 0.8
    )
    radialGrad.addColorStop(0, 'rgba(0, 242, 254, 0.04)')
    radialGrad.addColorStop(0.5, 'rgba(10, 15, 29, 0.2)')
    radialGrad.addColorStop(1, 'rgba(5, 7, 14, 0.8)')
    ctx.fillStyle = radialGrad
    ctx.fillRect(0, 0, width, height)

    // 绘制下半部科技透视网格
    gridOffset = (gridOffset + 0.4) % 40
    ctx.save()
    ctx.strokeStyle = 'rgba(0, 242, 254, 0.05)'
    ctx.lineWidth = 1

    // 纵向网格线（微透视）
    const vSteps = 24
    for (let i = 0; i <= vSteps; i++) {
      const x = (i / vSteps) * width
      ctx.beginPath()
      ctx.moveTo(x, 0)
      ctx.lineTo(x, height)
      ctx.stroke()
    }

    // 横向网格线（动态推进）
    for (let y = gridOffset; y < height; y += 40) {
      ctx.beginPath()
      ctx.moveTo(0, y)
      ctx.lineTo(width, y)
      ctx.stroke()
    }
    ctx.restore()

    // 粒子连线与更新
    for (let i = 0; i < particles.length; i++) {
      const p = particles[i]

      // 鼠标斥力微交互
      const dx = mouseX - p.x
      const dy = mouseY - p.y
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < 120 && dist > 0) {
        const force = (120 - dist) / 120
        p.x -= (dx / dist) * force * 1.5
        p.y -= (dy / dist) * force * 1.5
      }

      p.x += p.vx
      p.y += p.vy

      if (p.x < 0) p.x = width
      if (p.x > width) p.x = 0
      if (p.y < 0) p.y = height
      if (p.y > height) p.y = 0

      // 绘制粒子
      ctx.beginPath()
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
      ctx.fillStyle = p.color
      ctx.globalAlpha = p.alpha
      ctx.shadowBlur = 8
      ctx.shadowColor = p.color
      ctx.fill()
      ctx.shadowBlur = 0
      ctx.globalAlpha = 1

      // 与临近粒子神经连线
      for (let j = i + 1; j < particles.length; j++) {
        const p2 = particles[j]
        const pjDist = Math.hypot(p.x - p2.x, p.y - p2.y)
        if (pjDist < 90) {
          ctx.beginPath()
          ctx.moveTo(p.x, p.y)
          ctx.lineTo(p2.x, p2.y)
          ctx.strokeStyle = p.color
          ctx.globalAlpha = (1 - pjDist / 90) * 0.15
          ctx.stroke()
          ctx.globalAlpha = 1
        }
      }
    }

    animationId = requestAnimationFrame(render)
  }

  render()

  onBeforeUnmount(() => {
    if (animationId) cancelAnimationFrame(animationId)
    window.removeEventListener('resize', handleResize)
    window.removeEventListener('mousemove', handleMouseMove)
  })
})
</script>

<template>
  <canvas
    ref="canvasRef"
    class="pointer-events-none fixed inset-0 z-0 h-full w-full opacity-90"
  />
</template>
