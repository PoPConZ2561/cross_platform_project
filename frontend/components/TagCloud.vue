<template>
  <div class="cloud" role="list">
    <button
      v-for="t in tags" :key="t.name" role="listitem"
      :class="{ on: t.name === selected }"
      :style="{ fontSize: size(t.count) + 'rem' }"
      @click="$emit('select', t.name)"
    >{{ t.name }}<sup>{{ t.count }}</sup></button>
    <p v-if="!tags.length" class="empty">ยังไม่มี Tag</p>
  </div>
</template>

<script setup lang="ts">
import type { TagCount } from '~/composables/useBackend'
const props = defineProps<{ tags: TagCount[]; selected?: string }>()
defineEmits<{ select: [name: string] }>()

// ขนาดตัวอักษรเป็นสัดส่วนกับจำนวนรูปที่ใช้ Tag นั้น (0.9 – 2.4 rem)
const size = (c: number) => {
  const counts = props.tags.map(t => t.count)
  const min = Math.min(...counts), max = Math.max(...counts)
  return max === min ? 1.3 : 0.9 + (1.5 * (c - min)) / (max - min)
}
</script>
