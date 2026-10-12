<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import DevicePlayer from '@/components/player/DevicePlayer.vue'

const route = useRoute()
const serial = computed(() => typeof route.query.serial === 'string' ? route.query.serial : '')
</script>

<template>
  <DevicePlayer :key="serial" :serial="serial" />
</template>
