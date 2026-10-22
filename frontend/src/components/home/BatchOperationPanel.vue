<script setup lang="ts">
import type { Device } from '@/api/devices'
import type { DashSize } from '@/composables/useHomePhoneView'
import type { BatchItemResult, BatchSummary } from '@/composables/useDeviceBatch'
import BatchPanelShell from './batch/BatchPanelShell.vue'
import BatchDeviceGrid from './batch/BatchDeviceGrid.vue'
import BatchTargetScope from './batch/BatchTargetScope.vue'
import BatchActionButtons from './batch/BatchActionButtons.vue'
import BatchDisplaySettings from './batch/BatchDisplaySettings.vue'
import BatchResultList from './batch/BatchResultList.vue'

defineProps<{
  isOpen: boolean
  viewMode: 'deck' | 'compact' | 'phone'
  devices: Device[]
  hiddenSerials: string[]
  selectedSerials: string[]
  displayName: (d: Device) => string
  selectedCount: number
  isAllSelected: boolean
  hasSelectable: boolean
  batchRunning: boolean
  currentAction: string
  batchResults: BatchItemResult[]
  batchSummary: BatchSummary
  dashSize?: DashSize
  isPageFullscreen?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:isOpen', val: boolean): void
  (e: 'toggleHide', serial: string): void
  (e: 'showAll'): void
  (e: 'showOnlySelected'): void
  (e: 'selectAll'): void
  (e: 'clearSelection'): void
  (e: 'triggerAction', action: 'back' | 'home' | 'app_switch'): void
  (e: 'cancelBatch'): void
  (e: 'clearResults'): void
  (e: 'update:dashSize', val: DashSize): void
  (e: 'togglePageFullscreen'): void
}>()
</script>

<template>
  <BatchPanelShell v-if="isOpen" @close="emit('update:isOpen', false)">
    <!-- 编号设备显隐小方块选择区 -->
    <BatchDeviceGrid
      :devices="devices"
      :hidden-serials="hiddenSerials"
      :selected-serials="selectedSerials"
      :display-name="displayName"
      @toggle-hide="emit('toggleHide', $event)"
      @show-all="emit('showAll')"
      @show-only-selected="emit('showOnlySelected')"
    />

    <!-- 批量选择范围 -->
    <BatchTargetScope
      :selected-count="selectedCount"
      :is-all-selected="isAllSelected"
      :has-selectable="hasSelectable"
      :batch-running="batchRunning"
      @select-all="emit('selectAll')"
      @clear-selection="emit('clearSelection')"
    />

    <!-- 批量动作 -->
    <BatchActionButtons
      :batch-running="batchRunning"
      :current-action="currentAction"
      :selected-count="selectedCount"
      @trigger-action="emit('triggerAction', $event)"
      @cancel-batch="emit('cancelBatch')"
    />

    <!-- 显示设置 -->
    <BatchDisplaySettings
      :view-mode="viewMode"
      :dash-size="dashSize"
      :is-page-fullscreen="isPageFullscreen"
      @update:dash-size="emit('update:dashSize', $event)"
      @toggle-page-fullscreen="emit('togglePageFullscreen')"
    />

    <!-- 逐台执行结果 -->
    <BatchResultList
      :batch-results="batchResults"
      :batch-summary="batchSummary"
      @clear-results="emit('clearResults')"
    />
  </BatchPanelShell>
</template>
