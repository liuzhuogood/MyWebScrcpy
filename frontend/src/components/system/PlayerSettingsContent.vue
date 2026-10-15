<script setup lang="ts">
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { usePlayerSettings } from '@/composables/usePlayerSettings'

const s = usePlayerSettings()
</script>

<template>
  <div class="space-y-4">
    <!-- 窗口与显示 -->
    <div class="space-y-3 rounded-lg border p-4">
      <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">窗口与显示</h3>
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium">小窗默认尺寸</p>
          <p class="text-xs text-muted-foreground">悬浮小窗打开时的初始画面宽度</p>
        </div>
        <Select :model-value="s.floatingSize.value" @update:model-value="v => s.floatingSize.value = v as any">
          <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="compact">紧凑 (320px)</SelectItem>
            <SelectItem value="normal">标准 (380px)</SelectItem>
            <SelectItem value="large">宽大 (450px)</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- 画质与性能 -->
    <div class="space-y-3 rounded-lg border p-4">
      <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">画质与性能</h3>
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium">视频传输码率</p>
          <p class="text-xs text-muted-foreground">码率越高画面越清晰，网络要求更高</p>
        </div>
        <Select :model-value="String(s.bitrate.value)" @update:model-value="v => s.bitrate.value = Number(v)">
          <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="2000000">2 Mbps (标清省流)</SelectItem>
            <SelectItem value="4000000">4 Mbps (高清推荐)</SelectItem>
            <SelectItem value="8000000">8 Mbps (超清画质)</SelectItem>
            <SelectItem value="16000000">16 Mbps (极致高码)</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">帧率限制</p>
          <p class="text-xs text-muted-foreground">控制投屏的最大刷新帧率</p>
        </div>
        <Select :model-value="String(s.maxFps.value)" @update:model-value="v => s.maxFps.value = Number(v)">
          <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="15">15 FPS (节能低占用)</SelectItem>
            <SelectItem value="30">30 FPS (标准流畅)</SelectItem>
            <SelectItem value="60">60 FPS (高刷极速)</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- 声音与常用交互 -->
    <div class="space-y-3 rounded-lg border p-4">
      <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">声音与交互</h3>
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-medium">点击注入模式</p>
          <p class="text-xs text-muted-foreground">模拟器或风控严格的应用用 HID 更不易被识别为自动点击</p>
        </div>
        <Select :model-value="s.injectMode.value" aria-label="点击注入模式" @update:model-value="v => s.injectMode.value = v as any">
          <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="sdk">标准注入（默认）</SelectItem>
            <SelectItem value="hid">HID 硬件模拟</SelectItem>
            <SelectItem value="adb">ADB 命令</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">自动打开音量</p>
          <p class="text-xs text-muted-foreground">连接后自动播放手机声音（关闭则默认静音）</p>
        </div>
        <Switch :model-value="s.autoAudio.value" aria-label="自动打开音量开关" @update:model-value="v => s.autoAudio.value = !!v" />
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">唤醒后尝试解锁</p>
          <p class="text-xs text-muted-foreground">屏幕点亮时自动上滑解锁（有密码仍需手动验证）</p>
        </div>
        <Switch :model-value="s.autoUnlockOnWake.value" aria-label="唤醒后尝试解锁开关" @update:model-value="v => s.autoUnlockOnWake.value = !!v" />
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">剪贴板双向同步</p>
          <p class="text-xs text-muted-foreground">在电脑与手机设备间实时同步剪贴板文本</p>
        </div>
        <Switch :model-value="s.clipboardSync.value" aria-label="剪贴板双向同步开关" @update:model-value="v => s.clipboardSync.value = !!v" />
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">显示触摸反馈点</p>
          <p class="text-xs text-muted-foreground">在投屏画面中高亮显示手指触摸点击位置</p>
        </div>
        <Switch :model-value="s.touchVisible.value" aria-label="显示触摸反馈点开关" @update:model-value="v => s.touchVisible.value = !!v" />
      </div>
      <div class="flex items-center justify-between gap-4 border-t pt-3">
        <div>
          <p class="text-sm font-medium">显示匹配绿框</p>
          <p class="text-xs text-muted-foreground">显示模板匹配和元素定位识别绿框</p>
        </div>
        <Switch :model-value="s.boxesVisible.value" aria-label="显示匹配绿框开关" @update:model-value="v => s.boxesVisible.value = !!v" />
      </div>
    </div>
  </div>
</template>
