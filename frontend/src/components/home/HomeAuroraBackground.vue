<script setup lang="ts">
// 全息极光与星芒网格背景：完美适配亮色与暗色双主题，支持桌面端透明度调节
</script>

<template>
  <div
    class="aurora-desktop-transparent pointer-events-none fixed inset-0 z-0 overflow-hidden bg-slate-100/90 dark:bg-[#060810]"
  >
    <!-- 极光光斑 1: 蓝紫设备流 -->
    <div
      class="absolute -top-32 -left-32 h-[520px] w-[520px] rounded-full blur-[130px] animate-pulse bg-gradient-to-tr from-indigo-500/20 via-purple-500/15 to-transparent dark:from-indigo-600/35 dark:via-purple-600/25"
      style="animation-duration: 8s"
    />

    <!-- 极光光斑 2: 青蓝全息弧 -->
    <div
      class="absolute top-1/4 -right-32 h-[600px] w-[600px] rounded-full blur-[150px] animate-pulse bg-gradient-to-bl from-cyan-400/20 via-blue-500/15 to-transparent dark:from-cyan-500/30 dark:via-blue-600/20"
      style="animation-duration: 11s"
    />

    <!-- 极光光斑 3: 底部微弱深紫弧 -->
    <div
      class="absolute -bottom-40 left-1/4 h-[550px] w-[550px] rounded-full blur-[140px] animate-pulse bg-gradient-to-t from-violet-400/15 via-fuchsia-400/10 to-transparent dark:from-violet-600/25 dark:via-fuchsia-600/15"
      style="animation-duration: 9s"
    />

    <!-- 星芒网格：亮色下为深灰微点，暗色下为晶体微点 -->
    <div
      class="absolute inset-0 text-slate-900/15 dark:text-white/20 opacity-80"
      style="background-image: radial-gradient(currentColor 1px, transparent 1px); background-size: 32px 32px;"
    />
  </div>
</template>
