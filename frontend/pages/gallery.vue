<template>
  <h1>คลังรูปและค้นหา</h1>
  <p class="sub">ค้นหารูปจาก Tag หรือคำอธิบายภาพ</p>

  <div class="tabs" role="tablist">
    <button :class="{ on: mode === 'tag' }" @click="setMode('tag')">ค้นหาจาก Tag</button>
    <button :class="{ on: mode === 'desc' }" @click="setMode('desc')">ค้นหาจากคำอธิบาย</button>
  </div>

  <section v-if="mode === 'tag'" class="panel">
    <h2>Tag ทั้งหมด</h2>
    <TagCloud :tags="tags" :selected="selectedTag" @select="pickTag" />
  </section>

  <section v-else class="panel">
    <form class="row" @submit.prevent="searchDesc">
      <input v-model="query" type="text" placeholder="พิมพ์คำที่อยู่ในคำอธิบายภาพ เช่น ทะเล" aria-label="คำค้นหา" />
      <button type="submit">ค้นหา</button>
    </form>
  </section>

  <h2>{{ heading }}</h2>
  <p v-if="loading" class="empty">กำลังโหลด…</p>
  <ImageGrid v-else :items="photos" :empty-text="emptyText" @open="current = $event" />

  <div v-if="current" class="modal" @click.self="current = null">
    <div class="box" role="dialog" aria-modal="true">
      <img :src="imageUrl(current.id, false)" :alt="current.description || current.fileName" />
      <div class="info">
        <h2>{{ current.fileName }}</h2>
        <p>{{ current.description || 'ยังไม่มีคำอธิบาย' }}</p>
        <div class="chips">
          <button v-for="t in current.tags" :key="t" class="chip" style="border:0;cursor:pointer" @click="jumpToTag(t)">{{ t }}</button>
        </div>
        <button class="ghost" @click="current = null">ปิด</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PhotoItem, TagCount } from '~/composables/useBackend'
const { listImages, searchByTag, searchByDescription, tagCloud, imageUrl } = useBackend()

const mode = ref<'tag' | 'desc'>('tag')
const tags = ref<TagCount[]>([])
const photos = ref<PhotoItem[]>([])
const selectedTag = ref('')
const query = ref('')
const loading = ref(false)
const current = ref<PhotoItem | null>(null)
const searched = ref(false)

const heading = computed(() =>
  mode.value === 'tag' && selectedTag.value ? `รูปที่มี Tag “${selectedTag.value}” (${photos.value.length})`
  : searched.value ? `ผลการค้นหา (${photos.value.length})`
  : `รูปทั้งหมด (${photos.value.length})`)
const emptyText = computed(() => searched.value ? 'ไม่พบรูปที่ตรงกับคำค้น ลองใช้คำอื่น' : 'ยังไม่มีรูป ไปที่หน้า “สำรองรูป” เพื่อเริ่มต้น')

async function load(fn: () => Promise<PhotoItem[]>) {
  loading.value = true
  try { photos.value = await fn() } finally { loading.value = false }
}
async function setMode(m: 'tag' | 'desc') {
  mode.value = m; selectedTag.value = ''; query.value = ''; searched.value = false
  await load(listImages)
}
async function pickTag(name: string) {
  selectedTag.value = name; searched.value = true
  await load(() => searchByTag(name))
}
async function jumpToTag(name: string) {
  current.value = null; mode.value = 'tag'
  await pickTag(name)
}
async function searchDesc() {
  const q = query.value.trim()
  if (!q) return setMode('desc')
  searched.value = true
  await load(() => searchByDescription(q))
}

onMounted(async () => {
  tags.value = await tagCloud()
  await load(listImages)
})
</script>
