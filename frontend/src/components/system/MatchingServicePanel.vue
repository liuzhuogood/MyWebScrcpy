<script setup lang="ts">
import MatchingServiceContent from './MatchingServiceContent.vue'
import { useMatchingService } from '@/composables/useMatchingService'
const { status, error, busy, action } = useMatchingService()
</script>
<template><MatchingServiceContent :status="status" :busy="busy" :error="error" @action="action" /></template>
