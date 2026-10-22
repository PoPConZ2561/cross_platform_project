<template>
  <div v-if="items.length" class="grid">
    <button v-for="p in items" :key="p.id" class="card" @click="$emit('open', p)">
      <img :src="imageUrl(p.id)" :alt="p.description || p.fileName" loading="lazy" />
      <span>{{ p.fileName }}</span>
    </button>
  </div>
  <p v-else class="empty">{{ emptyText }}</p>
</template>

<script setup lang="ts">
import type { PhotoItem } from '~/composables/useBackend'
defineProps<{ items: PhotoItem[]; emptyText: string }>()
defineEmits<{ open: [p: PhotoItem] }>()
const { imageUrl } = useBackend()
</script>
