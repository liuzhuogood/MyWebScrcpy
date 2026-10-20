<script setup lang="ts">
// 空间全息渐变极光背景
</script>

<template>
  <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden bg-[#060810]">
    <!-- 极光光斑 1: 蓝紫空间流 -->
    <div
      class="absolute -top-32 -left-32 h-[500px] w-[500px] rounded-full bg-gradient-to-tr from-indigo-600/20 via-purple-600/15 to-transparent blur-[120px] animate-pulse"
      style="animation-duration: 8s"
    />

    <!-- 极光光斑 2: 青蓝全息弧 -->
    <div
      class="absolute top-1/3 -right-32 h-[600px] w-[600px] rounded-full bg-gradient-to-bl from-cyan-500/15 via-blue-600/10 to-transparent blur-[140px] animate-pulse"
      style="animation-duration: 11s"
    />

    <!-- 极光光斑 3: 底部微弱深紫弧 -->
    <div
      class="absolute -bottom-40 left-1/4 h-[550px] w-[550px] rounded-full bg-gradient-to-t from-violet-600/15 via-fuchsia-600/10 to-transparent blur-[130px] animate-pulse"
      style="animation-duration: 9s"
    />

    <!-- 精细全息微点星芒网格 -->
    <div
      class="absolute inset-0 opacity-[0.15]"
      style="background-image: radial-gradient(rgba(255, 255, 255, 0.4) 1px, transparent 1px); background-size: 32px 32px;"
    />
  </div>
</template>
